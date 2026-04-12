package commands

import (
	"context"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const statsCommandFallback = "Usage statistics and activity views are not available in Claude Code Go yet. Aggregated usage history, activity panels, and analytics-backed summaries remain unmigrated."

// StatsCommand exposes the minimum text-only /stats behavior available before usage dashboards exist in the Go host.
type StatsCommand struct{}

// Metadata returns the canonical slash descriptor for /stats.
func (c StatsCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "stats",
		Description: "Show your Claude Code usage statistics and activity",
		Usage:       "/stats",
	}
}

// Execute reports the stable usage-statistics fallback supported by the current Go host.
func (c StatsCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx
	_ = args

	logger.DebugCF("commands", "rendered stats command fallback output", map[string]any{
		"usage_statistics_available": false,
		"activity_view_available":    false,
	})

	return command.Result{
		Output: statsCommandFallback,
	}, nil
}
