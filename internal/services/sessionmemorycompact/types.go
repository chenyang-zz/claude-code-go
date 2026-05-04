package sessionmemorycompact

// SessionMemoryCompactConfig holds the configuration thresholds for session
// memory compaction.
type SessionMemoryCompactConfig struct {
	// MinTokens is the minimum number of tokens to preserve after compaction.
	MinTokens int
	// MinTextBlockMessages is the minimum number of messages with text blocks
	// to keep after compaction.
	MinTextBlockMessages int
	// MaxTokens is the maximum number of tokens to preserve after compaction
	// (hard cap).
	MaxTokens int
}
