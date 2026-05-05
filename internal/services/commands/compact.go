package commands

import (
	"context"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// CompactFunc is the function signature for executing a compaction operation.
// It receives custom instructions extracted from the command arguments and
// returns a user-facing result message. The function is injected at bootstrap
// time so the command layer stays decoupled from the compact infrastructure.
type CompactFunc func(ctx context.Context, customInstructions string) (string, error)

// CompactCommand implements the /compact slash command. When CompactFunc is
// set, it delegates to the real compaction path; otherwise it reports that
// compaction is not yet wired up.
type CompactCommand struct {
	// CompactFunc is the optional compaction implementation. When nil,
	// the command reports the fallback message.
	CompactFunc CompactFunc
}

// Metadata returns the canonical slash descriptor for /compact.
func (c CompactCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "compact",
		Description: "Compact conversation history, keeping a summary in context",
		Usage:       "/compact [instructions]",
	}
}

// Execute runs the /compact command. If a CompactFunc is wired, it extracts
// custom instructions from the arguments and delegates to the compaction path.
// Otherwise it reports the current fallback status.
func (c CompactCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	if c.CompactFunc == nil {
		logger.DebugCF("commands", "rendered compact command fallback output", map[string]any{
			"compact_available": false,
		})
		return command.Result{
			Output: "Conversation compaction is not available yet. Use /clear to start a new session.",
		}, nil
	}

	customInstructions := strings.TrimSpace(strings.Join(args.Raw, " "))
	result, err := c.CompactFunc(ctx, customInstructions)
	if err != nil {
		logger.DebugCF("commands", "compact command failed", map[string]any{
			"error": err.Error(),
		})
		return command.Result{
			Output: "Error compacting conversation: " + err.Error(),
		}, nil
	}

	return command.Result{
		Output: result,
	}, nil
}
