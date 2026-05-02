package sessionmemory

import (
	"context"
	"os"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// IsSessionMemoryEnabled checks whether the session memory feature is enabled.
// It checks the CLAUDE_FEATURE_SESSION_MEMORY env var; defaults to enabled
// when unset since session memory is a core product capability.
func IsSessionMemoryEnabled() bool {
	v := strings.ToLower(os.Getenv("CLAUDE_FEATURE_SESSION_MEMORY"))
	if v == "0" || v == "false" || v == "no" || v == "disabled" {
		return false
	}
	if v == "1" || v == "true" || v == "yes" || v == "enabled" {
		return true
	}
	return true
}

// SubagentRunner is the interface for running a forked subagent
// that executes the session memory update (e.g., FileEditTool).
// When nil, the extraction pipeline performs setup and builds the
// prompt but skips the subagent execution step.
type SubagentRunner interface {
	// Run executes a forked agent with the given messages.
	// The messages contain the user prompt that instructs the agent
	// to update the session memory file using FileEditTool.
	Run(ctx context.Context, messages []message.Message) error
}

// extractSessionMemory is the core post-turn hook function that checks
// thresholds and triggers session memory extraction when needed.
// It is registered as a PostTurnHook in InitSessionMemory.
func extractSessionMemory(ctx context.Context, messages []message.Message, runner SubagentRunner, querySource string) error {
	// Only run on main REPL thread.
	if querySource != "repl_main_thread" && querySource != "" {
		return nil
	}

	// Feature gate: skip if session memory is disabled.
	if !IsSessionMemoryEnabled() {
		return nil
	}

	// Lazy config init: use defaults (no remote config to wait on).
	// Config is already initialized by default.

	// Check extraction threshold.
	if !ShouldExtractMemory(messages, estimateTokens) {
		return nil
	}

	MarkExtractionStarted()
	defer MarkExtractionCompleted()

	// Set up the memory file (create dir, read current content).
	memoryPath, currentMemory, err := SetupSessionMemoryFile(ctx)
	if err != nil {
		logger.WarnCF("sessionmemory", "failed to setup memory file", map[string]any{
			"error": err.Error(),
		})
		return nil
	}

	// Build the update prompt with current notes and path.
	prompt := BuildSessionMemoryUpdatePrompt(currentMemory, memoryPath)

	if runner != nil {
		// Build a minimal user message containing the prompt.
		promptMsg := message.Message{
			Role: message.RoleUser,
			Content: []message.ContentPart{
				{Type: "text", Text: prompt},
			},
		}

		if err := runner.Run(ctx, []message.Message{promptMsg}); err != nil {
			logger.WarnCF("sessionmemory", "subagent execution failed", map[string]any{
				"error": err.Error(),
			})
			return nil
		}
	} else {
		logger.DebugCF("sessionmemory", "subagent runner is nil, skipping extraction execution", map[string]any{
			"memory_path": memoryPath,
		})
	}

	// Record extraction token count for next threshold check.
	RecordExtractionTokenCount(estimateTokens(messages))

	logger.DebugCF("sessionmemory", "extraction completed", map[string]any{
		"memory_path": memoryPath,
	})

	return nil
}

// estimateTokens provides a rough token estimation for message slices.
// Uses message count * 200 as a heuristic estimate when the compact
// package's TokenEstimation is not directly available.
// This is used for threshold comparisons and will be refined.
func estimateTokens(messages []message.Message) int {
	// A simple heuristic: estimate ~200 tokens per message on average.
	return len(messages) * 200
}
