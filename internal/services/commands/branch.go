package commands

import (
	"context"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const branchCommandFallback = "Interactive conversation branching is not available through /branch in Claude Code Go yet. Use `cc --fork-session --continue <prompt>` to fork the latest conversation in the current project, or `cc --fork-session /resume <session-id> <prompt>` to fork a specific saved session. Transcript-based branch naming and in-session resume handoff remain unmigrated."

// BranchCommand exposes the minimum text-only /branch behavior before transcript forking and interactive branch switching exist in the Go runtime.
type BranchCommand struct{}

// Metadata returns the canonical slash descriptor for /branch.
func (c BranchCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "branch",
		Aliases:     []string{"fork"},
		Description: "Create a branch of the current conversation at this point",
		Usage:       "/branch [name]",
	}
}

// Execute reports the stable branch fallback supported by the current Go host.
func (c BranchCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx
	_ = args

	logger.DebugCF("commands", "rendered branch command fallback output", map[string]any{
		"interactive_branching_available": false,
		"fork_session_hint_available":     true,
	})

	return command.Result{
		Output: branchCommandFallback,
	}, nil
}
