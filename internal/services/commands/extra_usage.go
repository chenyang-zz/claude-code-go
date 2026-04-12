package commands

import (
	"context"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const extraUsageCommandFallback = "Extra usage enrollment is not available in Claude Code Go yet. Browser launch, account overage management, and post-enrollment login handoff remain unmigrated."

// ExtraUsageCommand exposes the minimum text-only /extra-usage behavior available before browser and account flows exist in the Go host.
type ExtraUsageCommand struct{}

// Metadata returns the canonical slash descriptor for /extra-usage.
func (c ExtraUsageCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "extra-usage",
		Description: "Configure extra usage to keep working when limits are hit",
		Usage:       "/extra-usage",
	}
}

// Execute reports the stable extra-usage fallback supported by the current Go host.
func (c ExtraUsageCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx
	_ = args

	logger.DebugCF("commands", "rendered extra-usage command fallback output", map[string]any{
		"browser_launch_supported": false,
		"overage_management":       false,
		"login_handoff_supported":  false,
	})

	return command.Result{
		Output: extraUsageCommandFallback,
	}, nil
}
