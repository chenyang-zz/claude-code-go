package file_read

import (
	"fmt"
	"os"
	"strconv"
	"sync"
)

const (
	// DEFAULT_MAX_OUTPUT_TOKENS is the default post-read token cap for text
	// and notebook reads. Aligns with the TS limits.ts constant.
	DEFAULT_MAX_OUTPUT_TOKENS = 25000

	// envMaxTokensOverride is the environment variable that allows users to
	// override the default max output tokens.
	envMaxTokensOverride = "CLAUDE_CODE_FILE_READ_MAX_OUTPUT_TOKENS"
)

// FileReadingLimits defines the output caps for the Read tool.
type FileReadingLimits struct {
	// MaxTokens caps the estimated token count of returned content.
	MaxTokens int
	// MaxSizeBytes caps the total file size before reading.
	MaxSizeBytes int64
}

var (
	defaultLimitsOnce sync.Once
	defaultLimits     FileReadingLimits
)

// getDefaultFileReadingLimits returns the default file reading limits.
// The result is computed once and cached to avoid mid-session cap changes.
func getDefaultFileReadingLimits() FileReadingLimits {
	defaultLimitsOnce.Do(func() {
		maxTokens := DEFAULT_MAX_OUTPUT_TOKENS
		if envVal := getEnvMaxTokens(); envVal > 0 {
			maxTokens = envVal
		}
		defaultLimits = FileReadingLimits{
			MaxTokens:    maxTokens,
			MaxSizeBytes: defaultMaxFileSizeBytes,
		}
	})
	return defaultLimits
}

// getEnvMaxTokens reads the CLAUDE_CODE_FILE_READ_MAX_OUTPUT_TOKENS env var.
// Returns 0 when unset or invalid so callers can fall through to defaults.
func getEnvMaxTokens() int {
	override := os.Getenv(envMaxTokensOverride)
	if override == "" {
		return 0
	}
	parsed, err := strconv.Atoi(override)
	if err != nil || parsed <= 0 {
		return 0
	}
	return parsed
}

// bytesPerTokenForExtension returns the estimated bytes-per-token ratio
// for a given file extension. Dense JSON has many single-character tokens
// (e.g. {, }, :, ,, ") which makes the real ratio closer to 2 rather than
// the default 4.
func bytesPerTokenForExtension(ext string) int {
	switch ext {
	case "json", "jsonl", "jsonc":
		return 2
	default:
		return 4
	}
}

// estimateTokensForContent returns a rough token count for file content
// using a file-type-aware bytes-per-token ratio.
func estimateTokensForContent(content string, ext string) int {
	if content == "" {
		return 0
	}
	ratio := bytesPerTokenForExtension(ext)
	return (len(content) + ratio - 1) / ratio
}

// MaxFileReadTokenExceededError is returned when file content exceeds
// the maximum allowed tokens.
type MaxFileReadTokenExceededError struct {
	// TokenCount is the estimated token count that exceeded the limit.
	TokenCount int
	// MaxTokens is the configured maximum allowed tokens.
	MaxTokens int
}

// Error returns the user-facing error message aligned with the TS side.
func (e *MaxFileReadTokenExceededError) Error() string {
	return fmt.Sprintf(
		"File content (%d tokens) exceeds maximum allowed tokens (%d). Use offset and limit parameters to read specific portions of the file, or search for specific content instead of reading the whole file.",
		e.TokenCount, e.MaxTokens,
	)
}

// validateContentTokens checks whether content exceeds the token limit.
// It uses rough estimation (no API call) and returns an error if exceeded.
func validateContentTokens(content string, ext string, maxTokens int) error {
	if maxTokens <= 0 {
		maxTokens = getDefaultFileReadingLimits().MaxTokens
	}
	estimate := estimateTokensForContent(content, ext)
	if estimate <= maxTokens {
		return nil
	}
	return &MaxFileReadTokenExceededError{
		TokenCount: estimate,
		MaxTokens:  maxTokens,
	}
}
