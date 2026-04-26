package commands

import (
	"context"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const commitCommandFallback = "Prompt-driven git commit automation is not available in Claude Code Go yet. Prompt shell execution, attribution injection, and allowed-tool wiring remain unmigrated."

// CommitCommand exposes the minimum text-only /commit behavior before prompt automation exists in the Go host.
type CommitCommand struct{}

// Metadata returns the canonical slash descriptor for /commit.
func (c CommitCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "commit",
		Description: "Create a git commit",
		Usage:       "/commit",
	}
}

// Execute accepts optional free-form guidance and reports the stable /commit fallback.
func (c CommitCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx

	raw := strings.TrimSpace(args.RawLine)

	logger.DebugCF("commands", "rendered commit command fallback output", map[string]any{
		"commit_automation_available": false,
		"arg_provided":                raw != "",
	})

	return command.Result{
		Output: commitCommandFallback,
	}, nil
}
