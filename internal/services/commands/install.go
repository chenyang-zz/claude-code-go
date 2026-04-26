package commands

import (
	"context"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const installCommandFallback = "Native installer flow is not available in Claude Code Go yet. Platform package installation, shell integration setup, and force/channel handling remain unmigrated."

// InstallCommand exposes the minimum text-only /install behavior before native installer integrations exist in the Go runtime.
type InstallCommand struct{}

// Metadata returns the canonical slash descriptor for /install.
func (c InstallCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "install",
		Description: "Install Claude Code native build",
		Usage:       "/install [options]",
	}
}

// Execute reports the stable /install fallback supported by the current Go host.
func (c InstallCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx
	_ = args

	logger.DebugCF("commands", "rendered install command fallback output", map[string]any{
		"native_install_available": false,
	})

	return command.Result{
		Output: installCommandFallback,
	}, nil
}
