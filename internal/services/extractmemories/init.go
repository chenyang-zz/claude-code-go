package extractmemories

import (
	"context"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// PostTurnHookFunc matches the engine.PostTurnHook signature for registration.
type PostTurnHookFunc func(ctx context.Context, messages []message.Message, workingDir string) error

// InitExtractMemories initializes the extractMemories system by registering
// a PostTurnHook that triggers memory extraction after each complete turn.
//
// The runner is a SubagentRunner for forked agent execution (pass nil to skip
// subagent execution — prompt building and tracking still work).
// The registerPostTurnHook callback hooks into the engine's post-turn system.
// projectRoot is the root directory of the current project.
func InitExtractMemories(runner SubagentRunner, registerPostTurnHook func(hook PostTurnHookFunc), projectRoot string) *System {
	if !IsExtractMemoriesEnabled() {
		logger.DebugCF("extractmemories", "skipping init: extractMemories disabled", nil)
		return nil
	}

	if !IsAutoMemoryEnabled() {
		logger.DebugCF("extractmemories", "skipping init: auto memory disabled", nil)
		return nil
	}

	if runner == nil {
		logger.DebugCF("extractmemories", "initializing without subagent runner (detection/tracking only)", nil)
	}

	sys := NewSystem(runner, projectRoot)

	// Register PostTurnHook — trigger extraction after each complete turn.
	registerPostTurnHook(PostTurnHookFunc(func(ctx context.Context, messages []message.Message, workingDir string) error {
		return sys.extractAfterTurn(ctx, messages)
	}))

	logger.DebugCF("extractmemories", "extractMemories initialized", map[string]any{
		"project_root": projectRoot,
	})

	return sys
}
