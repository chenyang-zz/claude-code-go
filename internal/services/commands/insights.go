package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const insightsCommandFallback = "Insights report generation is not available in Claude Code Go yet. Session analytics pipelines and report rendering remain unmigrated."

// InsightsCommand exposes the minimum text-only /insights behavior before usage report generation exists in the Go host.
type InsightsCommand struct{}

// Metadata returns the canonical slash descriptor for /insights.
func (c InsightsCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "insights",
		Description: "Generate a report analyzing your Claude Code sessions",
		Usage:       "/insights [--homespaces]",
	}
}

// Execute accepts optional --homespaces argument and reports the stable /insights fallback.
func (c InsightsCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx

	raw := strings.TrimSpace(args.RawLine)
	if raw != "" && raw != "--homespaces" {
		return command.Result{}, fmt.Errorf("usage: %s", c.Metadata().Usage)
	}

	logger.DebugCF("commands", "rendered insights command fallback output", map[string]any{
		"insights_available":       false,
		"collect_remote_requested": raw == "--homespaces",
	})

	return command.Result{
		Output: insightsCommandFallback,
	}, nil
}
