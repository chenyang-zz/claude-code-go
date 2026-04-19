package compact

import (
	"encoding/json"
	"fmt"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
)

// defaultBytesPerToken is the character-to-token heuristic ratio used for
// local token estimation. Matches the TS roughTokenCountEstimation default.
const defaultBytesPerToken = 4

// EstimateTokens returns a rough token count for a slice of conversation
// messages using character-based heuristics (~4 chars/token).
// This aligns with the TS roughTokenCountEstimationForMessages function and
// is used for local auto-compact trigger decisions.
func EstimateTokens(messages []message.Message) int {
	total := 0
	for i := range messages {
		total += estimateMessageTokens(&messages[i])
	}
	return total
}

// estimateMessageTokens estimates the token count for a single message by
// summing token estimates across all its content blocks.
func estimateMessageTokens(msg *message.Message) int {
	if msg == nil {
		return 0
	}
	total := 0
	for i := range msg.Content {
		total += estimateBlockTokens(&msg.Content[i])
	}
	return total
}

// estimateBlockTokens estimates the token count for a single content block
// based on its type.
func estimateBlockTokens(block *message.ContentPart) int {
	if block == nil {
		return 0
	}
	switch block.Type {
	case "text":
		return roughEstimate(block.Text)
	case "tool_use":
		input, _ := json.Marshal(block.ToolInput)
		return roughEstimate(block.ToolName + string(input))
	case "tool_result":
		return roughEstimate(block.Text)
	default:
		// Unknown block types: estimate from text if present.
		if block.Text != "" {
			return roughEstimate(block.Text)
		}
		return 0
	}
}

// roughEstimate converts a character count to an approximate token count
// using the default bytes-per-token ratio.
func roughEstimate(text string) int {
	if text == "" {
		return 0
	}
	return (len(text) + defaultBytesPerToken - 1) / defaultBytesPerToken
}

// EstimateTokensForText returns a rough token estimate for a raw string.
func EstimateTokensForText(text string) int {
	return roughEstimate(text)
}

// FormatTokenCount produces a human-readable summary of token usage for
// logging and debugging.
func FormatTokenCount(tokenCount, threshold, windowSize int) string {
	return fmt.Sprintf("tokens=%d threshold=%d effectiveWindow=%d", tokenCount, threshold, windowSize)
}
