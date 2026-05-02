package sessionmemory

import (
	"fmt"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
	"github.com/sheepzhao/claude-code-go/internal/core/tool"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// fileEditToolName is the stable registry name for the FileEditTool
// ("Edit"). Matching this name mirrors the TS-side
// FILE_EDIT_TOOL_NAME constant check in createMemoryFileCanUseTool.
const fileEditToolName = "Edit"

// TokenEstimateFunc estimates the token count of a message slice.
type TokenEstimateFunc func(messages []message.Message) int

// HasMetInitializationThreshold reports whether the current context token count
// meets the minimum threshold to initialize session memory extraction.
// Returns true when currentTokenCount >= config.MinimumMessageTokensToInit.
// Mirrors the TS hasMetInitializationThreshold (sessionMemoryUtils.ts:173-177).
func HasMetInitializationThreshold(currentTokenCount int) bool {
	cfg := GetSessionMemoryConfig()
	met := currentTokenCount >= cfg.MinimumMessageTokensToInit
	logger.DebugCF("sessionmemory", "initialization threshold check", map[string]any{
		"current_token_count": currentTokenCount,
		"minimum_tokens_init": cfg.MinimumMessageTokensToInit,
		"met":                 met,
	})
	return met
}

// HasMetUpdateThreshold reports whether the context has grown enough since the
// last extraction to warrant another session memory update. Uses the same
// token-growth metric (currentTokenCount - tokensAtLastExtraction) as the TS
// hasMetUpdateThreshold (sessionMemoryUtils.ts:184-189).
func HasMetUpdateThreshold(currentTokenCount int) bool {
	cfg := GetSessionMemoryConfig()
	tokensAtLastExtraction := GetTokensAtLastExtraction()
	growth := currentTokenCount - tokensAtLastExtraction
	met := growth >= cfg.MinimumTokensBetweenUpdate
	logger.DebugCF("sessionmemory", "update threshold check", map[string]any{
		"current_token_count":       currentTokenCount,
		"tokens_at_last_extraction": tokensAtLastExtraction,
		"growth":                    growth,
		"minimum_tokens_update":     cfg.MinimumTokensBetweenUpdate,
		"met":                       met,
	})
	return met
}

// GetToolCallsBetweenUpdates returns the configured number of tool calls that
// should occur between session memory updates. Mirrors the TS
// getToolCallsBetweenUpdates (sessionMemoryUtils.ts:194-196).
func GetToolCallsBetweenUpdates() int {
	cfg := GetSessionMemoryConfig()
	return cfg.ToolCallsBetweenUpdates
}

// CountToolCallsSince counts tool_use content blocks in assistant messages that
// appear after the message identified by sinceMessageID. When sinceMessageID is
// empty, counting starts from the first message in the slice.
//
// Note: precise message matching by UUID requires a UUID field on the
// message.Message struct, which is not currently present in this codebase.
// When sinceMessageID is non-empty, tool calls are counted from the beginning
// as a safe fallback. This means the tool call threshold will always reflect
// total cumulative tool calls rather than calls since the last extraction.
// The token threshold is always required, so this does not cause runaway
// extractions.
//
// Mirrors the TS countToolCallsSince (sessionMemory.ts:108-132).
func CountToolCallsSince(messages []message.Message, sinceMessageID string) int {
	toolCallCount := 0
	foundStart := sinceMessageID == ""

	for _, msg := range messages {
		if !foundStart {
			// message.Message does not currently carry a UUID field for
			// matching against sinceMessageID. Start counting from the
			// first message as a safe default.
			foundStart = true
		}

		if msg.Role == message.RoleAssistant {
			for _, part := range msg.Content {
				if part.Type == "tool_use" {
					toolCallCount++
				}
			}
		}
	}

	logger.DebugCF("sessionmemory", "tool calls counted", map[string]any{
		"since_message_id": sinceMessageID,
		"tool_call_count":  toolCallCount,
	})
	return toolCallCount
}

// HasToolCallsInLastAssistantTurn reports whether the most recent assistant
// message in the conversation contains any tool_use content blocks.
// Mirrors the TS hasToolCallsInLastAssistantTurn imported from utils/messages.
func HasToolCallsInLastAssistantTurn(messages []message.Message) bool {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == message.RoleAssistant {
			for _, part := range messages[i].Content {
				if part.Type == "tool_use" {
					return true
				}
			}
			// Found the last assistant message; no tool calls in it.
			return false
		}
	}
	return false
}

// ShouldExtractMemory is the main decision function that determines whether a
// session memory extraction should be performed. The decision logic is:
//
//	1. If session memory is not yet initialized:
//	   a. If the token estimate does not meet MinimumMessageTokensToInit,
//	      return false.
//	   b. Otherwise, mark session memory as initialized.
//	2. Check token growth threshold: hasMetTokenThreshold.
//	3. Count tool calls since the last summarized message.
//	4. Check tool call threshold: hasMetToolCallThreshold.
//	5. Check if the last assistant turn has tool calls.
//	6. shouldExtract =
//	     (hasMetTokenThreshold && hasMetToolCallThreshold) ||
//	     (hasMetTokenThreshold && !hasToolCallsInLastTurn)
//	7. Token threshold is ALWAYS required.
//
// When extraction is decided, the last summarized message ID is updated to
// the UUID of the last message (requires a UUID field on message.Message).
//
// Mirrors the TS shouldExtractMemory (sessionMemory.ts:134-181).
func ShouldExtractMemory(messages []message.Message, tokenEstimate TokenEstimateFunc) bool {
	if len(messages) == 0 {
		logger.DebugCF("sessionmemory", "should extract memory: no messages", nil)
		return false
	}

	currentTokenCount := tokenEstimate(messages)

	// Step 1: Initialization check
	if !IsSessionMemoryInitialized() {
		if !HasMetInitializationThreshold(currentTokenCount) {
			logger.DebugCF("sessionmemory", "should extract memory: below init threshold", nil)
			return false
		}
		MarkSessionMemoryInitialized()
		logger.InfoCF("sessionmemory", "session memory initialized", map[string]any{
			"token_count": currentTokenCount,
		})
	}

	// Step 2: Token growth threshold
	hasMetTokenThreshold := HasMetUpdateThreshold(currentTokenCount)

	// Step 3: Tool calls since last summarized message
	toolCallsSince := CountToolCallsSince(messages, GetLastSummarizedMessageID())

	// Step 4: Tool call threshold
	hasMetToolCallThreshold := toolCallsSince >= GetToolCallsBetweenUpdates()

	// Step 5: Last turn has no tool calls
	hasNoToolCallsInLastTurn := !HasToolCallsInLastAssistantTurn(messages)

	// Step 6: Combined decision
	shouldExtract := (hasMetTokenThreshold && hasMetToolCallThreshold) ||
		(hasMetTokenThreshold && hasNoToolCallsInLastTurn)

	logger.DebugCF("sessionmemory", "extraction decision", map[string]any{
		"current_token_count":         currentTokenCount,
		"has_met_token_threshold":     hasMetTokenThreshold,
		"tool_calls_since":            toolCallsSince,
		"has_met_tool_call_threshold": hasMetToolCallThreshold,
		"has_no_tool_calls_last_turn": hasNoToolCallsInLastTurn,
		"should_extract":              shouldExtract,
	})

	// Step 7: Update last summarized message ID when extracting.
	// Note: This requires a UUID field on message.Message for precise
	// tracking. Without it, SetLastSummarizedMessageID is not called.
	if shouldExtract {
		logger.InfoCF("sessionmemory", "session memory extraction triggered", map[string]any{
			"token_count": currentTokenCount,
		})
	}

	return shouldExtract
}

// CreateMemoryFileCanUseTool returns a closure that checks whether a tool
// invocation is allowed for session memory operations. Only the FileEditTool
// ("Edit") is allowed, and only for the exact memoryPath. All other tools and
// paths are denied.
//
// The returned function signature is
//
//	func(tool tool.Tool, input map[string]any) (allowed bool, reason string)
//
// The caller is responsible for translating this into the appropriate
// Decision type.
//
// Mirrors the TS createMemoryFileCanUseTool (sessionMemory.ts:460-482).
func CreateMemoryFileCanUseTool(memoryPath string) func(tool.Tool, map[string]any) (bool, string) {
	return func(t tool.Tool, input map[string]any) (bool, string) {
		// Only allow the FileEditTool.
		if t.Name() != fileEditToolName {
			return false, fmt.Sprintf("only %s tool is allowed for session memory operations", fileEditToolName)
		}

		// The input must contain a file_path field.
		rawPath, ok := input["file_path"]
		if !ok {
			return false, "missing file_path in tool input"
		}

		filePath, ok := rawPath.(string)
		if !ok || filePath != memoryPath {
			return false, fmt.Sprintf("only edits on %s are allowed for session memory", memoryPath)
		}

		return true, ""
	}
}
