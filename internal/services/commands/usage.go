package commands

import (
	"context"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const usageCommandFallback = "Plan usage limits UI is not available in Claude Code Go yet. Settings Usage tab rendering, account plan limit lookup, and consumer subscription state detection remain unmigrated."

// UsageCommand exposes the minimum text-only /usage behavior available before the settings usage view exists in the Go host.
type UsageCommand struct{}

// Metadata returns the canonical slash descriptor for /usage.
func (c UsageCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "usage",
		Description: "Show plan usage limits",
		Usage:       "/usage",
	}
}

// Execute reports the stable usage-limit fallback supported by the current Go host.
func (c UsageCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx
	_ = args

	logger.DebugCF("commands", "rendered usage command fallback output", map[string]any{
		"usage_limits_ui_available": false,
		"plan_limit_lookup":         false,
	})

	return command.Result{
		Output: usageCommandFallback,
	}, nil
}
