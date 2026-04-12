package commands

import (
	"context"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const upgradeURL = "https://claude.ai/upgrade/max"

const upgradeCommandFallback = "Interactive upgrade flow is not available in Claude Code Go yet. Review Claude Max plans at " + upgradeURL + ". Browser launch, subscription detection, and post-upgrade login handoff remain unmigrated."

// UpgradeCommand exposes the minimum text-only /upgrade behavior available before browser integration exists in the Go host.
type UpgradeCommand struct{}

// Metadata returns the canonical slash descriptor for /upgrade.
func (c UpgradeCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "upgrade",
		Description: "Upgrade to Max for higher rate limits and more Opus",
		Usage:       "/upgrade",
	}
}

// Execute reports the stable upgrade fallback supported by the current Go host.
func (c UpgradeCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx
	_ = args

	logger.DebugCF("commands", "rendered upgrade command fallback output", map[string]any{
		"browser_launch_supported": false,
		"login_handoff_supported":  false,
	})

	return command.Result{
		Output: upgradeCommandFallback,
	}, nil
}
