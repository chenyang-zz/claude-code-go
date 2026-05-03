package commands

import (
	"context"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/internal/services/policylimits"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const webSetupCommandFallback = "Web setup is not available in Claude Code Go yet. Claude web onboarding, GitHub token import, and browser handoff flows remain unmigrated."

// WebSetupCommand exposes the minimum text-only /web-setup behavior before web onboarding exists in the Go host.
type WebSetupCommand struct{}

// Metadata returns the canonical slash descriptor for /web-setup.
func (c WebSetupCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "web-setup",
		Description: "Setup Claude Code on the web (requires connecting your GitHub account)",
		Usage:       "/web-setup",
	}
}

// Execute accepts no required arguments and reports the stable /web-setup fallback.
func (c WebSetupCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx

	if allowed, reason := policylimits.IsAllowed(policylimits.ActionAllowRemoteSessions); !allowed {
		return command.Result{Output: reason}, nil
	}

	raw := strings.TrimSpace(args.RawLine)

	logger.DebugCF("commands", "rendered web-setup command fallback output", map[string]any{
		"web_setup_available": false,
		"arg_provided":        raw != "",
	})

	return command.Result{
		Output: webSetupCommandFallback,
	}, nil
}
