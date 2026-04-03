package executor

// Limits groups runtime guardrails that constrain executor behavior.
type Limits struct {
	// MaxToolOutputBytes caps tool output size to avoid unbounded runtime buffers.
	MaxToolOutputBytes int
}
