package commands

import (
	"context"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const installSlackAppCommandFallback = "Slack App installation is not available in Claude Code Go yet. Workspace authorization, app provisioning, and interactive setup remain unmigrated."

// InstallSlackAppCommand exposes the minimum text-only /install-slack-app behavior before Slack host integrations exist in the Go runtime.
type InstallSlackAppCommand struct{}

// Metadata returns the canonical slash descriptor for /install-slack-app.
func (c InstallSlackAppCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "install-slack-app",
		Description: "Install the Claude Slack app",
		Usage:       "/install-slack-app",
	}
}

// Execute reports the stable /install-slack-app fallback supported by the current Go host.
func (c InstallSlackAppCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx
	_ = args

	logger.DebugCF("commands", "rendered install-slack-app command fallback output", map[string]any{
		"slack_app_install_available": false,
	})

	return command.Result{
		Output: installSlackAppCommandFallback,
	}, nil
}
