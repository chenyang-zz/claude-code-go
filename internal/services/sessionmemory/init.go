package sessionmemory

import (
	"context"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// PostTurnHookFunc matches engine.PostTurnHook signature for registration.
type PostTurnHookFunc func(ctx context.Context, messages []message.Message, workingDir string) error

// InitSessionMemory initializes the session memory system by registering
// a PostTurnHook that triggers memory extraction after each complete turn.
//
// The registerPostTurnHook callback hooks into the engine's post-turn system.
// The runner is a SubagentRunner for forked agent execution (pass nil to skip).
func InitSessionMemory(runner SubagentRunner, registerPostTurnHook func(hook PostTurnHookFunc)) {
	if !IsSessionMemoryEnabled() {
		logger.DebugCF("sessionmemory", "skipping init: session memory disabled", nil)
		return
	}

	if runner == nil {
		logger.DebugCF("sessionmemory", "initializing without subagent runner (detection/tracking only)", nil)
	}

	// Register PostTurnHook — check thresholds and extract after each complete turn.
	registerPostTurnHook(PostTurnHookFunc(func(ctx context.Context, messages []message.Message, workingDir string) error {
		return extractSessionMemory(ctx, messages, runner, "repl_main_thread")
	}))

	logger.DebugCF("sessionmemory", "session memory initialized", nil)
}

// ResetLastSummarizedMessageID clears the last summarized message ID.
// Used externally by the compact system when compaction replaces messages.
func ResetLastSummarizedMessageID() {
	SetLastSummarizedMessageID("")
}
