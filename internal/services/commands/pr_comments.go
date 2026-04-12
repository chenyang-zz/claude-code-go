package commands

import (
	"context"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const prCommentsPluginNotice = "This command has moved to a plugin.\n\n1. Install it with: claude plugin install pr-comments@claude-code-marketplace\n2. Then run: /pr-comments:pr-comments\n3. More information: https://github.com/anthropics/claude-code-marketplace/blob/main/pr-comments/README.md"

// PRCommentsCommand exposes the migrated /pr-comments entrypoint as a stable plugin guidance notice.
type PRCommentsCommand struct{}

// Metadata returns the canonical slash descriptor for /pr-comments.
func (c PRCommentsCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "pr-comments",
		Description: "Get comments from a GitHub pull request",
		Usage:       "/pr-comments",
	}
}

// Execute reports the stable plugin migration notice used by the source command.
func (c PRCommentsCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx
	_ = args

	logger.DebugCF("commands", "rendered pr-comments plugin migration notice", map[string]any{
		"command": "pr-comments",
	})

	return command.Result{
		Output: prCommentsPluginNotice,
	}, nil
}
