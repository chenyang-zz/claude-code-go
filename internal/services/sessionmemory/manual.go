package sessionmemory

import (
	"context"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// ManualExtractionResult holds the outcome of a manual extraction.
type ManualExtractionResult struct {
	Success   bool
	MemoryPath string
	Error     string
}

// ManuallyExtractSessionMemory performs a manual session memory extraction,
// bypassing all threshold checks. Used by the /summary command.
func ManuallyExtractSessionMemory(ctx context.Context, messages []message.Message) ManualExtractionResult {
	if len(messages) == 0 {
		return ManualExtractionResult{
			Success: false,
			Error:   "No messages to summarize",
		}
	}

	MarkExtractionStarted()
	defer MarkExtractionCompleted()

	// Set up the memory file (create dir, read current content).
	memoryPath, currentMemory, err := SetupSessionMemoryFile(ctx)
	if err != nil {
		logger.WarnCF("sessionmemory", "manual extraction: failed to setup memory file", map[string]any{
			"error": err.Error(),
		})
		return ManualExtractionResult{
			Success: false,
			Error:   err.Error(),
		}
	}

	// Build the update prompt with current notes and path.
	prompt := BuildSessionMemoryUpdatePrompt(currentMemory, memoryPath)

	// Build a user message containing the prompt.
	_ = message.Message{
		Role: message.RoleUser,
		Content: []message.ContentPart{
			message.TextPart(prompt),
		},
	}

	// Note: Subagent execution is not yet wired. The prompt is built and
	// ready for subagent execution when a SubagentRunner is available.
	logger.DebugCF("sessionmemory", "manual extraction prompt ready (subagent not wired)", map[string]any{
		"memory_path": memoryPath,
		"prompt_len":  len(prompt),
	})

	// Record extraction token count for next threshold check.
	RecordExtractionTokenCount(estimateTokens(messages))

	return ManualExtractionResult{
		Success:    true,
		MemoryPath: memoryPath,
	}
}
