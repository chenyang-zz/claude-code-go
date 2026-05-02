package sessionmemory

import (
	"context"
	"sync"
	"time"
)

// SessionMemoryConfig holds thresholds for session memory extraction decisions.
// All values must be positive; zero values trigger fallback to defaults.
type SessionMemoryConfig struct {
	// MinimumMessageTokensToInit is the minimum context window tokens before
	// session memory is initialized. Default: 10000.
	MinimumMessageTokensToInit int
	// MinimumTokensBetweenUpdate is the minimum context window growth (tokens)
	// between session memory updates. Default: 5000.
	MinimumTokensBetweenUpdate int
	// ToolCallsBetweenUpdates is the number of tool calls between updates.
	// Default: 3.
	ToolCallsBetweenUpdates int
}

// DefaultSessionMemoryConfig returns the default session memory configuration.
func DefaultSessionMemoryConfig() SessionMemoryConfig {
	return SessionMemoryConfig{
		MinimumMessageTokensToInit: 10000,
		MinimumTokensBetweenUpdate: 5000,
		ToolCallsBetweenUpdates:    3,
	}
}

const (
	// extractionWaitTimeout is how long to wait for an in-progress extraction.
	extractionWaitTimeout = 15 * time.Second
	// extractionStaleThreshold marks an extraction as stale after this duration.
	extractionStaleThreshold = 60 * time.Second
	// extractionPollInterval is the sleep interval between polls.
	extractionPollInterval = 1 * time.Second
)

// State holds the mutable session memory extraction state.
type State struct {
	mu sync.Mutex

	config SessionMemoryConfig

	extractionStartedAt    time.Time
	tokensAtLastExtraction int
	sessionMemoryInitialized bool
	lastSummarizedMessageID string
}

var globalState State

func init() {
	globalState.config = DefaultSessionMemoryConfig()
}

// SetSessionMemoryConfig replaces the current config.
func SetSessionMemoryConfig(cfg SessionMemoryConfig) {
	globalState.mu.Lock()
	defer globalState.mu.Unlock()
	globalState.config = cfg
}

// GetSessionMemoryConfig returns a copy of the current config.
func GetSessionMemoryConfig() SessionMemoryConfig {
	globalState.mu.Lock()
	defer globalState.mu.Unlock()
	return globalState.config
}

// MarkExtractionStarted records the start time of an extraction.
func MarkExtractionStarted() {
	globalState.mu.Lock()
	defer globalState.mu.Unlock()
	globalState.extractionStartedAt = time.Now()
}

// MarkExtractionCompleted clears the extraction start time.
func MarkExtractionCompleted() {
	globalState.mu.Lock()
	defer globalState.mu.Unlock()
	globalState.extractionStartedAt = time.Time{}
}

// IsExtractionInProgress returns true if an extraction is currently running.
func IsExtractionInProgress() bool {
	globalState.mu.Lock()
	defer globalState.mu.Unlock()
	return !globalState.extractionStartedAt.IsZero()
}

// WaitForSessionMemoryExtraction blocks until any in-progress extraction
// completes, or until the timeout or stale threshold is reached.
func WaitForSessionMemoryExtraction(ctx context.Context) error {
	start := time.Now()
	for {
		globalState.mu.Lock()
		startedAt := globalState.extractionStartedAt
		globalState.mu.Unlock()

		if startedAt.IsZero() {
			return nil
		}
		if time.Since(startedAt) > extractionStaleThreshold {
			return nil // stale, don't wait
		}
		if time.Since(start) > extractionWaitTimeout {
			return nil // timeout exceeded
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(extractionPollInterval):
		}
	}
}

// RecordExtractionTokenCount records the context token count at extraction time.
func RecordExtractionTokenCount(tokens int) {
	globalState.mu.Lock()
	defer globalState.mu.Unlock()
	globalState.tokensAtLastExtraction = tokens
}

// GetTokensAtLastExtraction returns the token count recorded at last extraction.
func GetTokensAtLastExtraction() int {
	globalState.mu.Lock()
	defer globalState.mu.Unlock()
	return globalState.tokensAtLastExtraction
}

// MarkSessionMemoryInitialized marks session memory as having been initialized.
func MarkSessionMemoryInitialized() {
	globalState.mu.Lock()
	defer globalState.mu.Unlock()
	globalState.sessionMemoryInitialized = true
}

// IsSessionMemoryInitialized reports whether session memory has been initialized.
func IsSessionMemoryInitialized() bool {
	globalState.mu.Lock()
	defer globalState.mu.Unlock()
	return globalState.sessionMemoryInitialized
}

// SetLastSummarizedMessageID sets the UUID of the last summarized message.
func SetLastSummarizedMessageID(id string) {
	globalState.mu.Lock()
	defer globalState.mu.Unlock()
	globalState.lastSummarizedMessageID = id
}

// GetLastSummarizedMessageID returns the UUID of the last summarized message.
func GetLastSummarizedMessageID() string {
	globalState.mu.Lock()
	defer globalState.mu.Unlock()
	return globalState.lastSummarizedMessageID
}

// ResetState resets all session memory state (used in testing).
func ResetState() {
	globalState.mu.Lock()
	defer globalState.mu.Unlock()
	globalState.config = DefaultSessionMemoryConfig()
	globalState.extractionStartedAt = time.Time{}
	globalState.tokensAtLastExtraction = 0
	globalState.sessionMemoryInitialized = false
	globalState.lastSummarizedMessageID = ""
}
