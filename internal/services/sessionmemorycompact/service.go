package sessionmemorycompact

import (
	"os"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/compact"
	"github.com/sheepzhao/claude-code-go/internal/core/message"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// hasTextBlocks checks whether a message contains text content blocks.
// For user messages, checks for non-empty string content or text-type content
// blocks. For assistant messages, checks for text-type content blocks.
func hasTextBlocks(msg message.Message) bool {
	if msg.Role == message.RoleAssistant {
		for _, block := range msg.Content {
			if block.Type == "text" {
				return true
			}
		}
		return false
	}
	if msg.Role == message.RoleUser {
		for _, block := range msg.Content {
			if block.Type == "text" && block.Text != "" {
				return true
			}
		}
		return false
	}
	return false
}

// getToolResultIDs collects all tool_use_id values from tool_result blocks
// in a user message.
func getToolResultIDs(msg message.Message) []string {
	if msg.Role != message.RoleUser {
		return nil
	}
	var ids []string
	for _, block := range msg.Content {
		if block.Type == "tool_result" && block.ToolUseID != "" {
			ids = append(ids, block.ToolUseID)
		}
	}
	return ids
}

// hasToolUseWithIDs checks whether an assistant message contains tool_use
// blocks matching any of the given ids.
func hasToolUseWithIDs(msg message.Message, toolUseIDs map[string]struct{}) bool {
	if msg.Role != message.RoleAssistant {
		return false
	}
	for _, block := range msg.Content {
		if block.Type == "tool_use" && block.ToolUseID != "" {
			if _, ok := toolUseIDs[block.ToolUseID]; ok {
				return true
			}
		}
	}
	return false
}

// AdjustIndexToPreserveAPIInvariants adjusts the start index to ensure
// tool_use/tool_result pairs are not split and thinking blocks sharing the
// same message ID with kept assistant messages are included.
//
// This corresponds to TS adjustIndexToPreserveAPIInvariants and handles:
// 1. Collecting tool_result IDs from all kept messages and looking backwards
//    for assistant messages with matching tool_use blocks.
// 2. Collecting message IDs from kept assistant messages and looking backwards
//    for messages with the same ID (which may contain thinking blocks).
func AdjustIndexToPreserveAPIInvariants(messages []message.Message, startIndex int) int {
	if startIndex <= 0 || startIndex >= len(messages) {
		return startIndex
	}

	adjustedIndex := startIndex

	// Step 1: Handle tool_use/tool_result pairs
	// Collect tool_result IDs from ALL messages in the kept range
	var allToolResultIDs []string
	for i := startIndex; i < len(messages); i++ {
		allToolResultIDs = append(allToolResultIDs, getToolResultIDs(messages[i])...)
	}

	if len(allToolResultIDs) > 0 {
		// Collect tool_use IDs already in the kept range
		toolUseIDsInKeptRange := make(map[string]struct{})
		for i := adjustedIndex; i < len(messages); i++ {
			msg := messages[i]
			if msg.Role == message.RoleAssistant {
				for _, block := range msg.Content {
					if block.Type == "tool_use" && block.ToolUseID != "" {
						toolUseIDsInKeptRange[block.ToolUseID] = struct{}{}
					}
				}
			}
		}

		// Only look for tool_uses that are NOT already in the kept range
		neededToolUseIDs := make(map[string]struct{})
		for _, id := range allToolResultIDs {
			if _, ok := toolUseIDsInKeptRange[id]; !ok {
				neededToolUseIDs[id] = struct{}{}
			}
		}

		// Find the assistant message(s) with matching tool_use blocks
		for i := adjustedIndex - 1; i >= 0 && len(neededToolUseIDs) > 0; i-- {
			msg := messages[i]
			if hasToolUseWithIDs(msg, neededToolUseIDs) {
				adjustedIndex = i
				// Remove found tool_use IDs from the set
				if msg.Role == message.RoleAssistant {
					for _, block := range msg.Content {
						if block.Type == "tool_use" && block.ToolUseID != "" {
							delete(neededToolUseIDs, block.ToolUseID)
						}
					}
				}
			}
		}
	}

	// Step 2: Handle thinking blocks that share message ID with kept assistant messages.
	// In the TS version this uses message.message.id; the Go side currently lacks
	// this field, so this step is conservative — we don't check for shared IDs.
	// This may be refined when message identity tracking is added.
	_ = adjustedIndex // Step 2 deferred — Go side has no message.ID field

	return adjustedIndex
}

// CalculateMessagesToKeepIndex calculates the starting index for messages to
// keep after session memory compaction. Starts from lastSummarizedIndex, then
// expands backwards to meet minimum token and text-block message requirements.
//
// This corresponds to TS calculateMessagesToKeepIndex.
func CalculateMessagesToKeepIndex(messages []message.Message, lastSummarizedIndex int) int {
	if len(messages) == 0 {
		return 0
	}

	cfg := GetSessionMemoryCompactConfig()

	// Start from the message after lastSummarizedIndex
	startIndex := messagesLen(messages)
	if lastSummarizedIndex >= 0 && lastSummarizedIndex < len(messages)-1 {
		startIndex = lastSummarizedIndex + 1
	}

	// Calculate current tokens and text-block message count from startIndex to end
	totalTokens := 0
	textBlockMessageCount := 0
	for i := startIndex; i < len(messages); i++ {
		msgTokens := compact.EstimateTokens(messages[i : i+1])
		totalTokens += msgTokens
		if hasTextBlocks(messages[i]) {
			textBlockMessageCount++
		}
	}

	// Check if we already hit the max cap
	if totalTokens >= cfg.MaxTokens {
		return AdjustIndexToPreserveAPIInvariants(messages, startIndex)
	}

	// Check if we already meet both minimums
	if totalTokens >= cfg.MinTokens && textBlockMessageCount >= cfg.MinTextBlockMessages {
		return AdjustIndexToPreserveAPIInvariants(messages, startIndex)
	}

	// Expand backwards until we meet both minimums or hit max cap
	for i := startIndex - 1; i >= 0; i-- {
		msgTokens := compact.EstimateTokens(messages[i : i+1])
		totalTokens += msgTokens
		if hasTextBlocks(messages[i]) {
			textBlockMessageCount++
		}
		startIndex = i

		// Stop if we hit the max cap
		if totalTokens >= cfg.MaxTokens {
			break
		}

		// Stop if we meet both minimums
		if totalTokens >= cfg.MinTokens && textBlockMessageCount >= cfg.MinTextBlockMessages {
			break
		}

		// Stop if we've gone past all messages
		if i == 0 {
			break
		}
	}

	// Adjust for tool pairs
	return AdjustIndexToPreserveAPIInvariants(messages, startIndex)
}

// ShouldUseSessionMemoryCompaction checks whether session memory compaction
// should be used. Checks env overrides and the feature flag.
//
// This corresponds to TS shouldUseSessionMemoryCompaction (simplified:
// GrowthBook integration and analytics events are stubbed).
func ShouldUseSessionMemoryCompaction() bool {
	// Allow env override for eval runs and testing
	if isEnvTruthy("ENABLE_CLAUDE_CODE_SM_COMPACT") {
		return true
	}
	if isEnvTruthy("DISABLE_CLAUDE_CODE_SM_COMPACT") {
		return false
	}

	return IsSessionMemoryCompactEnabled()
}

// TrySessionMemoryCompaction attempts to use session memory for compaction.
// This is a simplified implementation that stubs the session memory file I/O
// and hook processing. Returns the compaction index and messages-to-keep, or
// nil when session memory compaction cannot be used.
//
// This corresponds to TS trySessionMemoryCompaction (simplified).
// TODO: wire up real session memory content when the session memory system
// integration is ready. Currently stubs session memory content as empty and
// falls back on returning nil (which means the caller should use legacy compact).
func TrySessionMemoryCompaction(messages []message.Message) *SessionMemoryCompactResult {
	if !ShouldUseSessionMemoryCompaction() {
		return nil
	}

	// Simplified: we always fall back to nil (no session memory content)
	// TODO: read real session memory content via the session memory service

	// Fallback: session memory not available
	logger.DebugCF("sessionmemorycompact", "session memory content not available, skipping", nil)
	return nil
}

// SessionMemoryCompactResult holds the result of a session memory compaction
// attempt. This corresponds to a subset of TS CompactionResult fields needed
// by the caller.
type SessionMemoryCompactResult struct {
	// MessagesToKeep are the messages preserved after compaction.
	MessagesToKeep []message.Message
	// StartIndex is the calculated start index for kept messages.
	StartIndex int
}

// getSessionMemoryContent is a stub for reading session memory content.
// TODO: wire up real session memory file reading.
func getSessionMemoryContent() (string, error) {
	return "", nil
}

// isEnvTruthy returns true if the environment variable is set to a truthy
// value ("1", "true", "yes", case-insensitive).
func isEnvTruthy(name string) bool {
	v := os.Getenv(name)
	switch strings.ToLower(v) {
	case "1", "true", "yes":
		return true
	}
	return false
}

// messagesLen returns the length of a message slice, defaulting to 0 for nil.
func messagesLen(msgs []message.Message) int {
	if msgs == nil {
		return 0
	}
	return len(msgs)
}
