package commands

import (
	"context"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const commitPushPRCommandFallback = "Prompt-driven commit/push/PR automation is not available in Claude Code Go yet. GitHub CLI integration, PR body generation, and external notification wiring remain unmigrated."

// CommitPushPRCommand exposes the minimum text-only /commit-push-pr behavior before prompt automation exists in the Go host.
type CommitPushPRCommand struct{}

// Metadata returns the canonical slash descriptor for /commit-push-pr.
func (c CommitPushPRCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "commit-push-pr",
		Description: "Commit, push, and open a PR",
		Usage:       "/commit-push-pr [instructions]",
	}
}

// Execute accepts optional user instructions and reports the stable /commit-push-pr fallback.
func (c CommitPushPRCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx

	raw := strings.TrimSpace(args.RawLine)

	logger.DebugCF("commands", "rendered commit-push-pr command fallback output", map[string]any{
		"commit_push_pr_available": false,
		"arg_provided":             raw != "",
	})

	return command.Result{
		Output: commitPushPRCommandFallback,
	}, nil
}
