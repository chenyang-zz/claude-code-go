package autodream

import (
	"context"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// PostTurnHookFunc matches the engine.PostTurnHook signature for registration.
type PostTurnHookFunc func(ctx context.Context, messages []message.Message, workingDir string) error

// InitAutoDream initializes the autoDream system by registering a PostTurnHook
// that triggers memory consolidation checks after each complete turn.
//
// The runner is a SubagentRunner for forked agent execution (pass nil to skip
// subagent execution — gate checks and lock management still work).
// The registerPostTurnHook callback hooks into the engine's post-turn system.
// projectRoot is the root directory of the current project.
func InitAutoDream(runner SubagentRunner, registerPostTurnHook func(hook PostTurnHookFunc), projectRoot string) *System {
	if !isAutoDreamEnabled() {
		logger.DebugCF("autodream", "skipping init: autoDream disabled", nil)
		return nil
	}

	sys := NewSystem(runner, projectRoot)

	// Register PostTurnHook — trigger consolidation check after each complete turn.
	registerPostTurnHook(PostTurnHookFunc(func(ctx context.Context, messages []message.Message, workingDir string) error {
		return sys.RunAutoDream(ctx)
	}))

	logger.DebugCF("autodream", "autoDream initialized", map[string]any{
		"project_root": projectRoot,
	})

	return sys
}
