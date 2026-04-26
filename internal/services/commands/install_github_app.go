package commands

import (
	"context"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const installGitHubAppCommandFallback = "GitHub App installation flow is not available in Claude Code Go yet. Repository selection, OAuth handoff, workflow bootstrap, and secret provisioning remain unmigrated."

// InstallGitHubAppCommand exposes the minimum text-only /install-github-app behavior before host-integrated setup exists in the Go runtime.
type InstallGitHubAppCommand struct{}

// Metadata returns the canonical slash descriptor for /install-github-app.
func (c InstallGitHubAppCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "install-github-app",
		Description: "Set up Claude GitHub Actions for a repository",
		Usage:       "/install-github-app",
	}
}

// Execute reports the stable /install-github-app fallback supported by the current Go host.
func (c InstallGitHubAppCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx
	_ = args

	logger.DebugCF("commands", "rendered install-github-app command fallback output", map[string]any{
		"github_app_install_available": false,
	})

	return command.Result{
		Output: installGitHubAppCommandFallback,
	}, nil
}
