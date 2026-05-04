package autocompact

// AutoCompactTrackingState captures the per-query-loop tracking state needed
// by the auto-compact circuit breaker and re-compaction detection. The caller
// threads this struct through successive query loop iterations so that the
// circuit breaker can skip futile retry attempts when the context window is
// irrecoverably over the limit.
type AutoCompactTrackingState struct {
	// Compacted reports whether a compaction has already occurred in this
	// query loop chain.
	Compacted bool
	// TurnCounter counts the number of turns since the last compaction.
	TurnCounter int
	// TurnID is a unique identifier for the current turn.
	TurnID string
	// ConsecutiveFailures tracks how many auto-compact attempts have failed
	// in a row. Reset on success. When this reaches MaxConsecutiveFailures
	// the circuit breaker stops retrying until the next session.
	ConsecutiveFailures int
}
