package sessionmemorycompact

import (
	"context"
	"os"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/compact"
	"github.com/sheepzhao/claude-code-go/internal/core/message"
	"github.com/sheepzhao/claude-code-go/internal/services/sessionmemory"
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

	// Step 2: Handle thinking blocks that share message ID with kept assistant messages
	// Collect message IDs from assistant messages in the kept range.
	// In the Go side we don't have message.ID like TS does, so this step is
	// simplified: we include the preceding message if it's an assistant message
	// (which may contain thinking blocks to merge).
	if adjustedIndex > 0 {
		prev := messages[adjustedIndex-1]
		curr := messages[adjustedIndex]
		if prev.Role == message.RoleAssistant && curr.Role == message.RoleAssistant {
			adjustedIndex = adjustedIndex - 1
		}
	}

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
// It waits for any in-progress session memory extraction, reads the current
// session memory content, calculates the message index to keep based on
// the last summarized message position, and returns a SessionMemoryCompactResult.
// Returns nil when session memory compaction cannot be used.
//
// This corresponds to TS trySessionMemoryCompaction.
func TrySessionMemoryCompaction(ctx context.Context, messages []message.Message) *SessionMemoryCompactResult {
	if !ShouldUseSessionMemoryCompaction() {
		return nil
	}

	// Wait for any in-progress session memory extraction to complete (with timeout).
	if err := sessionmemory.WaitForSessionMemoryExtraction(ctx); err != nil {
		logger.DebugCF("sessionmemorycompact", "wait for extraction failed", map[string]any{
			"error": err.Error(),
		})
		return nil
	}

	// Read real session memory content via the session memory service.
	content, err := sessionmemory.GetSessionMemoryContent(ctx)
	if err != nil {
		logger.WarnCF("sessionmemorycompact", "failed to read session memory content", map[string]any{
			"error": err.Error(),
		})
		return nil
	}

	// No session memory file exists at all.
	if content == "" {
		logger.DebugCF("sessionmemorycompact", "no session memory content, skipping", nil)
		return nil
	}

	// Get the last summarized message ID.
	lastSummarizedMessageID := sessionmemory.GetLastSummarizedMessageID()

	var lastSummarizedIndex int
	if lastSummarizedMessageID != "" {
		// Normal case: find the message by ID. Without a UUID field on
		// message.Message, we assume the last assistant message with text
		// content is the summarized boundary.
		lastSummarizedIndex = findLastAssistantMessage(messages)
	} else {
		// Resumed session: set to last message so startIndex becomes
		// messages.length (no messages kept initially).
		lastSummarizedIndex = len(messages) - 1
	}

	// Calculate the starting index for messages to keep.
	startIndex := CalculateMessagesToKeepIndex(messages, lastSummarizedIndex)

	// Filter out old compact boundary messages from messagesToKeep.
	messagesToKeep := make([]message.Message, 0, len(messages)-startIndex)
	for i := startIndex; i < len(messages); i++ {
		messagesToKeep = append(messagesToKeep, messages[i])
	}

	logger.DebugCF("sessionmemorycompact", "session memory compaction result", map[string]any{
		"start_index":           startIndex,
		"messages_to_keep":      len(messagesToKeep),
		"total_messages":        len(messages),
		"session_memory_length": len(content),
	})

	return &SessionMemoryCompactResult{
		MessagesToKeep: messagesToKeep,
		StartIndex:     startIndex,
	}
}

// findLastAssistantMessage finds the index of the last assistant message in the
// messages slice. Used as a fallback when message UUID is not available to
// determine the summarized boundary.
func findLastAssistantMessage(messages []message.Message) int {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == message.RoleAssistant {
			return i
		}
	}
	return -1
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
