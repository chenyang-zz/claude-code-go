// Package tokenestimation provides token estimation utilities for content
// of various types: text, messages, content blocks, and file-type-aware estimates.
//
// The estimation uses character-based heuristics (~4 chars/token by default)
// and does not require an external API or provider client.
// For API-based counting, use the provider client directly.
package tokenestimation

import (
	"encoding/json"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
)

// defaultBytesPerToken is the default character-to-token ratio used for
// local token estimation. Matches the TS roughTokenCountEstimation default.
const defaultBytesPerToken = 4

// defaultMaxImageTokens is the estimated token cost for image/document blocks.
// Matches the TS constant used in roughTokenCountEstimationForBlock.
const defaultMaxImageTokens = 2000

// EstimateTokens returns a rough token count for a slice of conversation
// messages using character-based heuristics.
func EstimateTokens(messages []message.Message) int {
	total := 0
	for i := range messages {
		total += estimateMessageTokens(&messages[i])
	}
	return total
}

// EstimateTokensForText returns a rough token estimate for a raw string.
func EstimateTokensForText(text string) int {
	return roughEstimate(text)
}

// EstimateTokensForFileType returns a rough token estimate with a file-type-aware
// bytes-per-token ratio. Dense formats like JSON use 2 bytes/token; others use 4.
func EstimateTokensForFileType(content string, fileExtension string) int {
	ratio := bytesPerTokenForFileType(fileExtension)
	return roughEstimateWithRatio(content, ratio)
}

// BytesPerTokenForFileType returns the estimated bytes-per-token ratio for a
// given file extension. JSON-like formats return 2; everything else returns 4.
func BytesPerTokenForFileType(fileExtension string) int {
	return bytesPerTokenForFileType(fileExtension)
}

// EstimateContentTokens estimates tokens for a single content block.
func EstimateContentTokens(block message.ContentPart) int {
	return estimateBlockTokens(&block)
}

// EstimateMessagesTokens estimates tokens for all messages in a slice.
// This is an alias for EstimateTokens.
func EstimateMessagesTokens(messages []message.Message) int {
	return EstimateTokens(messages)
}

// EstimateToolsTokens estimates tokens for a slice of tool definitions.
// Tool schemas (JSON) are estimated with the JSON-specific ratio of 2 bytes/token.
func EstimateToolsTokens(tools []map[string]any) int {
	if len(tools) == 0 {
		return 0
	}
	total := 0
	ratio := bytesPerTokenForFileType("json")
	for _, tool := range tools {
		data, err := json.Marshal(tool)
		if err == nil {
			total += roughEstimateWithRatio(string(data), ratio)
		}
	}
	return total
}

// roughEstimate converts a character count to an approximate token count
// using the default bytes-per-token ratio.
func roughEstimate(text string) int {
	return roughEstimateWithRatio(text, defaultBytesPerToken)
}

// roughEstimateWithRatio converts a character count to an approximate token count
// using the specified bytes-per-token ratio.
func roughEstimateWithRatio(text string, bytesPerToken int) int {
	if text == "" || bytesPerToken <= 0 {
		return 0
	}
	return (len(text) + bytesPerToken - 1) / bytesPerToken
}

// bytesPerTokenForFileType returns the bytes-per-token ratio for a given file
// extension. JSON-like formats use 2 (dense tokens); everything else uses 4.
func bytesPerTokenForFileType(fileExtension string) int {
	switch strings.ToLower(fileExtension) {
	case "json", "jsonl", "jsonc":
		return 2
	default:
		return defaultBytesPerToken
	}
}

// estimateMessageTokens estimates tokens for a single message.
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

// estimateBlockTokens estimates tokens for a single content block.
func estimateBlockTokens(block *message.ContentPart) int {
	if block == nil {
		return 0
	}
	switch block.Type {
	case "text":
		return roughEstimate(block.Text)
	case "image", "document":
		return defaultMaxImageTokens
	case "tool_use":
		input, _ := json.Marshal(block.ToolInput)
		return roughEstimate(block.ToolName + string(input))
	case "tool_result":
		return roughEstimate(block.Text)
	case "thinking":
		return roughEstimate(block.Text)
	case "redacted_thinking":
		return roughEstimate(block.Text)
	default:
		if block.Text != "" {
			return roughEstimate(block.Text)
		}
		if block.Data != "" {
			return roughEstimate(block.Data)
		}
		return 0
	}
}
