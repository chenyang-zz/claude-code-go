package commands

import (
	"context"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const securityReviewPluginNotice = "This command has moved to a plugin.\n\n1. Install it with: claude plugin install security-review@claude-code-marketplace\n2. Then run: /security-review:security-review\n3. More information: https://github.com/anthropics/claude-code-marketplace/blob/main/security-review/README.md"

// SecurityReviewCommand exposes the migrated /security-review entrypoint as a stable plugin guidance notice.
type SecurityReviewCommand struct{}

// Metadata returns the canonical slash descriptor for /security-review.
func (c SecurityReviewCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "security-review",
		Description: "Complete a security review of the pending changes on the current branch",
		Usage:       "/security-review",
	}
}

// Execute reports the stable plugin migration notice used by the source command.
func (c SecurityReviewCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx
	_ = args

	logger.DebugCF("commands", "rendered security-review plugin migration notice", map[string]any{
		"command": "security-review",
	})

	return command.Result{
		Output: securityReviewPluginNotice,
	}, nil
}
