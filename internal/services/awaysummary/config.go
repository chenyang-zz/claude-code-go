package awaysummary

import "time"

// DefaultIdleThreshold is the default idle duration before generating an away summary.
const DefaultIdleThreshold = 5 * time.Minute

// DefaultModel is the small/fast model used for away summary generation.
const DefaultModel = "claude-haiku-4-5-20251001"

// DefaultMaxMessages is the number of recent messages used as context.
const DefaultMaxMessages = 30

// Config holds configuration for the away summary system.
type Config struct {
	// IdleThreshold is the minimum idle duration before generating a summary.
	IdleThreshold time.Duration
	// Model is the model name used for summary generation.
	Model string
	// MaxMessages is the number of recent messages used for context.
	MaxMessages int
}

// DefaultConfig returns the default configuration.
func DefaultConfig() Config {
	return Config{
		IdleThreshold: DefaultIdleThreshold,
		Model:         DefaultModel,
		MaxMessages:   DefaultMaxMessages,
	}
}
