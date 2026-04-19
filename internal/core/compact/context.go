package compact

import (
	"os"
	"strconv"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
)

// ModelContextWindowDefault is the default context window size for models
// without an explicit override. Matches TS MODEL_CONTEXT_WINDOW_DEFAULT.
const ModelContextWindowDefault = 200_000

// AutoCompactBufferTokens is the token buffer reserved before the context
// window limit. Auto-compact triggers when token count reaches
// effectiveWindow - AutoCompactBufferTokens. Matches TS AUTOCOMPACT_BUFFER_TOKENS.
const AutoCompactBufferTokens = 13_000

// MaxOutputTokensForSummary caps the output token budget for compact summary
// generation. Matches TS MAX_OUTPUT_TOKENS_FOR_SUMMARY.
const MaxOutputTokensForSummary = 20_000

// MaxConsecutiveAutoCompactFailures is the circuit breaker threshold.
// After this many consecutive failures, auto-compact stops retrying.
// Matches TS MAX_CONSECUTIVE_AUTOCOMPACT_FAILURES.
const MaxConsecutiveAutoCompactFailures = 3

// modelContextWindows maps canonical model name substrings to their context
// window sizes. Order matters: first match wins.
var modelContextWindows = []struct {
	pattern string
	tokens  int
}{
	{"claude-opus-4", 200_000},
	{"claude-sonnet-4", 200_000},
	{"claude-haiku-4", 200_000},
}

// TrackingState tracks auto-compact state across the engine loop.
// Corresponds to TS AutoCompactTrackingState.
type TrackingState struct {
	// Compacted reports whether at least one auto-compact has occurred.
	Compacted bool
	// TurnCounter counts turns since the last compaction.
	TurnCounter int
	// TurnID is a unique identifier for the current turn.
	TurnID string
	// ConsecutiveFailures counts consecutive auto-compact failures,
	// reset on success. Used for circuit breaker.
	ConsecutiveFailures int
}

// GetContextWindowForModel returns the context window size for the given model.
// It checks the CLAUDE_CODE_AUTO_COMPACT_WINDOW env override first, then falls
// back to the per-model lookup table, then to ModelContextWindowDefault.
// Aligns with TS getContextWindowForModel.
func GetContextWindowForModel(model string) int {
	// Env override takes precedence.
	if envWindow := os.Getenv("CLAUDE_CODE_AUTO_COMPACT_WINDOW"); envWindow != "" {
		if parsed, err := strconv.Atoi(envWindow); err == nil && parsed > 0 {
			return parsed
		}
	}

	// Per-model lookup.
	lower := strings.ToLower(model)
	for _, entry := range modelContextWindows {
		if strings.Contains(lower, entry.pattern) {
			return entry.tokens
		}
	}
	return ModelContextWindowDefault
}

// GetEffectiveContextWindowSize returns the context window size minus the
// reserved output token budget for summary generation.
// Aligns with TS getEffectiveContextWindowSize.
func GetEffectiveContextWindowSize(model string) int {
	contextWindow := GetContextWindowForModel(model)
	reserved := min(MaxOutputTokensForSummary, maxOutputTokensForModel(model))
	return contextWindow - reserved
}

// GetAutoCompactThreshold returns the token count at which auto-compact
// should trigger: effectiveContextWindow - AutoCompactBufferTokens.
// Aligns with TS getAutoCompactThreshold.
func GetAutoCompactThreshold(model string) int {
	effectiveWindow := GetEffectiveContextWindowSize(model)
	return effectiveWindow - AutoCompactBufferTokens
}

// ShouldAutoCompact determines whether auto-compaction should be triggered
// for the given message list and model. It checks environment variable
// controls and compares the estimated token count against the threshold.
// Aligns with TS shouldAutoCompact.
func ShouldAutoCompact(messages []message.Message, model string) bool {
	if !IsAutoCompactEnabled() {
		return false
	}

	tokenCount := EstimateTokens(messages)
	threshold := GetAutoCompactThreshold(model)
	return tokenCount >= threshold
}

// ShouldAutoCompactWithTracking extends ShouldAutoCompact with circuit
// breaker support. It returns false when consecutive failures have reached
// the limit.
func ShouldAutoCompactWithTracking(messages []message.Message, model string, tracking *TrackingState) bool {
	if tracking != nil && tracking.ConsecutiveFailures >= MaxConsecutiveAutoCompactFailures {
		return false
	}
	return ShouldAutoCompact(messages, model)
}

// IsAutoCompactEnabled checks whether auto-compact is enabled via
// environment variables. Both DISABLE_COMPACT and DISABLE_AUTO_COMPACT
// disable auto-compact. Aligns with TS isAutoCompactEnabled.
func IsAutoCompactEnabled() bool {
	if isEnvTruthy("DISABLE_COMPACT") {
		return false
	}
	if isEnvTruthy("DISABLE_AUTO_COMPACT") {
		return false
	}
	return true
}

// maxOutputTokensForModel returns the maximum output tokens for a given model.
// For now, returns a conservative default. Can be extended with per-model
// lookup as needed.
func maxOutputTokensForModel(model string) int {
	return MaxOutputTokensForSummary
}

// isEnvTruthy returns true if the environment variable is set to a truthy
// value (1, true, yes). Case-insensitive.
func isEnvTruthy(key string) bool {
	val := strings.ToLower(os.Getenv(key))
	return val == "1" || val == "true" || val == "yes"
}
