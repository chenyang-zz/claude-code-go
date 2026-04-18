package model

// Usage carries token usage metrics from one model stream response.
type Usage struct {
	InputTokens              int
	OutputTokens             int
	CacheCreationInputTokens int
	CacheReadInputTokens     int
}

// Add returns a new Usage with each field summed.
func (u Usage) Add(other Usage) Usage {
	return Usage{
		InputTokens:              u.InputTokens + other.InputTokens,
		OutputTokens:             u.OutputTokens + other.OutputTokens,
		CacheCreationInputTokens: u.CacheCreationInputTokens + other.CacheCreationInputTokens,
		CacheReadInputTokens:     u.CacheReadInputTokens + other.CacheReadInputTokens,
	}
}

// IsZero reports whether all usage fields are zero.
func (u Usage) IsZero() bool {
	return u.InputTokens == 0 && u.OutputTokens == 0 &&
		u.CacheCreationInputTokens == 0 && u.CacheReadInputTokens == 0
}
