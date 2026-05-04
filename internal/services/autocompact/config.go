package autocompact

// MAX_OUTPUT_TOKENS_FOR_SUMMARY reserves this many tokens for output during
// compaction. Based on p99.99 of compact summary output being 17,387 tokens.
const MAX_OUTPUT_TOKENS_FOR_SUMMARY = 20_000

// AUTOCOMPACT_BUFFER_TOKENS is the token headroom between the effective context
// window and the auto-compact trigger threshold.
const AUTOCOMPACT_BUFFER_TOKENS = 13_000

// WARNING_THRESHOLD_BUFFER_TOKENS is the headroom used to compute the warning
// threshold (warning fires when remaining tokens fall below this value).
const WARNING_THRESHOLD_BUFFER_TOKENS = 20_000

// ERROR_THRESHOLD_BUFFER_TOKENS is the headroom used to compute the error
// threshold (error fires when remaining tokens fall below this value).
const ERROR_THRESHOLD_BUFFER_TOKENS = 20_000

// MANUAL_COMPACT_BUFFER_TOKENS is the headroom used to compute the blocking
// limit for manual compaction.
const MANUAL_COMPACT_BUFFER_TOKENS = 3_000

// MaxConsecutiveFailures is the circuit-breaker limit. After this many
// consecutive auto-compact failures the runtime stops retrying to avoid
// wasting API calls on irrecoverably over-limit contexts.
const MaxConsecutiveFailures = 3

// TokenWarningState holds the result of calculateTokenWarningState, describing
// how close the current token usage is to each threshold level.
type TokenWarningState struct {
	PercentLeft               int
	IsAboveWarningThreshold   bool
	IsAboveErrorThreshold     bool
	IsAboveAutoCompactThreshold bool
	IsAtBlockingLimit         bool
}

// CompactModelConfig holds model-specific parameters used by auto-compact
// threshold calculations.
type CompactModelConfig struct {
	// ContextWindow is the model's maximum context window in tokens.
	ContextWindow int
	// MaxOutputTokens is the model's maximum output tokens.
	MaxOutputTokens int
}
