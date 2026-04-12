package commands

import (
	"context"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const diffCommandFallback = "Interactive diff inspection is not available in Claude Code Go yet. Uncommitted change enumeration, per-turn diff views, and diff dialog rendering remain unmigrated."

// DiffCommand exposes the minimum text-only /diff behavior before diff inspection UI exists in the Go host.
type DiffCommand struct{}

// Metadata returns the canonical slash descriptor for /diff.
func (c DiffCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "diff",
		Description: "View uncommitted changes and per-turn diffs",
		Usage:       "/diff",
	}
}

// Execute reports the stable /diff fallback supported by the current Go host.
func (c DiffCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx
	_ = args

	logger.DebugCF("commands", "rendered diff command fallback output", map[string]any{
		"worktree_diff_available": false,
		"turn_diff_available":     false,
	})

	return command.Result{
		Output: diffCommandFallback,
	}, nil
}
