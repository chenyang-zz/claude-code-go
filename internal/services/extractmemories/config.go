// Package extractmemories implements automatic durable memory extraction from
// conversation history using a forked subagent that runs after each complete
// query loop.
//
// The extraction agent is a perfect fork of the main conversation — same
// system prompt, same message prefix. It runs with a restricted tool set
// (Read/Grep/Glob + read-only Bash + Edit/Write confined to the memory
// directory) and writes structured observations to per-topic markdown files
// under ~/.claude/projects/<path>/memory/.
package extractmemories

import (
	"os"
	"strings"
	"sync"
)

// FlagExtractMemories gates the extractMemories background extraction.
// Controlled by environment variable CLAUDE_FEATURE_EXTRACT_MEMORIES.
const FlagExtractMemories = "EXTRACT_MEMORIES"

const (
	// envPrefix is the CLAUDE_FEATURE_ prefix used for feature flags.
	envPrefix = "CLAUDE_FEATURE_"
)

// GrowthBook feature flag key constants for documentation and future
// GrowthBook SDK integration. Currently only the static env-var gate is used.
const (
	// GBTenguPassportQuail is the GrowthBook key for the extractMemories total gate.
	GBTenguPassportQuail = "tengu_passport_quail"
	// GBTenguBrambleLintel is the GrowthBook key for the extraction throttle interval.
	GBTenguBrambleLintel = "tengu_bramble_lintel"
	// GBTenguMothCopse is the GrowthBook key for skipping MEMORY.md index updates.
	GBTenguMothCopse = "tengu_moth_copse"
	// GBTenguSlateThimble is the GrowthBook key for enabling extraction in non-interactive mode.
	GBTenguSlateThimble = "tengu_slate_thimble"
)

// Defaults for extractMemories behavior.
const (
	// DefaultExtractIntervalTurns is the default number of turns between extractions.
	DefaultExtractIntervalTurns = 1
	// DefaultSkipIndex controls whether to skip MEMORY.md index updates by default.
	DefaultSkipIndex = false
	// DefaultDrainTimeoutMs is the default max wait for inflight extractions during drain.
	DefaultDrainTimeoutMs = 60000
	// DefaultMaxTurns is the max number of subagent turns per extraction run.
	DefaultMaxTurns = 5
)

// IsExtractMemoriesEnabled checks whether extractMemories is enabled via
// the CLAUDE_FEATURE_EXTRACT_MEMORIES env var. Defaults to enabled when unset.
func IsExtractMemoriesEnabled() bool {
	return isFeatureEnabled(FlagExtractMemories, true)
}

// isFeatureEnabled checks whether a named feature flag is enabled.
// When defaultValue is true, the flag defaults to enabled unless explicitly
// set to "0"/"false"/"no"/"disabled".
func isFeatureEnabled(name string, defaultValue bool) bool {
	v := strings.ToLower(os.Getenv(envPrefix + name))
	if v == "0" || v == "false" || v == "no" || v == "disabled" {
		return false
	}
	if v == "1" || v == "true" || v == "yes" || v == "enabled" {
		return true
	}
	return defaultValue
}

// ExtractionConfig holds the compile-time configuration for extractMemories.
type ExtractionConfig struct {
	// ExtractIntervalTurns is the minimum number of eligible turns between extractions.
	ExtractIntervalTurns int
	// SkipIndex controls whether to skip MEMORY.md index updates.
	SkipIndex bool
	// MaxTurns is the max subagent turns per extraction run.
	MaxTurns int
}

// DefaultExtractionConfig returns the default extraction configuration.
func DefaultExtractionConfig() ExtractionConfig {
	return ExtractionConfig{
		ExtractIntervalTurns: DefaultExtractIntervalTurns,
		SkipIndex:            DefaultSkipIndex,
		MaxTurns:             DefaultMaxTurns,
	}
}

// State tracks the mutable extraction state (cursor position, overlap guard,
// pending context). All fields are protected by the embedded mutex.
type State struct {
	mu sync.Mutex

	// config is the current extraction configuration.
	config ExtractionConfig

	// lastMessageIndex is the cursor — index of the last message processed.
	lastMessageIndex int

	// inProgress is true while an extraction is executing.
	inProgress bool

	// turnsSinceLastExtraction counts eligible turns since the last run.
	turnsSinceLastExtraction int

	// hasLoggedGateFailure avoids repeating the gate-disabled log message.
	hasLoggedGateFailure bool
}

// NewState creates a new extraction state with default config.
// The cursor starts at -1 (no extraction performed yet).
func NewState() *State {
	return &State{
		config:           DefaultExtractionConfig(),
		lastMessageIndex: -1,
	}
}

// SetConfig replaces the current extraction config.
func (s *State) SetConfig(cfg ExtractionConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config = cfg
}

// Config returns a copy of the current extraction config.
func (s *State) Config() ExtractionConfig {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.config
}

// GetLastMessageIndex returns the cursor index of the last message processed.
// Returns -1 when no extraction has been performed.
func (s *State) GetLastMessageIndex() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastMessageIndex
}

// SetLastMessageIndex advances the cursor to the given message index.
func (s *State) SetLastMessageIndex(idx int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastMessageIndex = idx
}

// IsInProgress reports whether an extraction is currently running.
func (s *State) IsInProgress() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.inProgress
}

// SetInProgress sets the in-progress flag.
func (s *State) SetInProgress(v bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.inProgress = v
}

// TurnsSinceLastExtraction returns the count of eligible turns since the last run.
func (s *State) TurnsSinceLastExtraction() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.turnsSinceLastExtraction
}

// IncrementTurnsSinceLastExtraction increments the turn counter.
func (s *State) IncrementTurnsSinceLastExtraction() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.turnsSinceLastExtraction++
}

// ResetTurnsSinceLastExtraction resets the turn counter to zero.
func (s *State) ResetTurnsSinceLastExtraction() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.turnsSinceLastExtraction = 0
}

// HasLoggedGateFailure reports whether we've already logged the gate-disabled message.
func (s *State) HasLoggedGateFailure() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.hasLoggedGateFailure
}

// SetHasLoggedGateFailure marks that the gate-disabled message has been logged.
func (s *State) SetHasLoggedGateFailure(v bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hasLoggedGateFailure = v
}

// Reset resets all mutable state (used in testing).
func (s *State) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config = DefaultExtractionConfig()
	s.lastMessageIndex = -1
	s.inProgress = false
	s.turnsSinceLastExtraction = 0
	s.hasLoggedGateFailure = false
}
