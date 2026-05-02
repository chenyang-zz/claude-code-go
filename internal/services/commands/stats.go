package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
	"github.com/sheepzhao/claude-code-go/internal/services/claudeailimits"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const statsCommandFallback = "Usage statistics and activity views are not available in Claude Code Go yet. Aggregated usage history, activity panels, and analytics-backed summaries remain unmigrated."

// StatsCommand exposes the minimum text-only /stats behavior available before usage dashboards exist in the Go host.
type StatsCommand struct {
	// Config carries the already-resolved runtime configuration snapshot.
	Config coreconfig.Config
	// Probe performs provider-specific quota checks when available.
	Probe UsageLimitsProber
}

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

	if snapshot, ok := c.snapshot(ctx); ok {
		logger.DebugCF("commands", "rendered stats command quota snapshot", map[string]any{
			"provider":       c.Config.Provider,
			"quota_status":   snapshot.Status,
			"overage_status": snapshot.OverageStatus,
		})
		return command.Result{
			Output: renderStatsSnapshot(snapshot),
		}, nil
	}

	logger.DebugCF("commands", "rendered stats command fallback output", map[string]any{
		"usage_statistics_available": false,
		"activity_view_available":    false,
	})

	return command.Result{
		Output: statsCommandFallback,
	}, nil
}

// snapshot probes the current provider when usage statistics observation is supported in the Go host.
func (c StatsCommand) snapshot(ctx context.Context) (UsageLimitsSnapshot, bool) {
	if coreconfig.NormalizeProvider(c.Config.Provider) != coreconfig.ProviderAnthropic {
		return UsageLimitsSnapshot{}, false
	}
	if missingUsageCredential(c.Config) || c.Probe == nil {
		return UsageLimitsSnapshot{}, false
	}
	return c.Probe.ProbeUsage(ctx, c.Config), true
}

// renderStatsSnapshot formats one Anthropic quota snapshot for `/stats`.
func renderStatsSnapshot(snapshot UsageLimitsSnapshot) string {
	lines := []string{
		"Anthropic usage activity snapshot:",
		fmt.Sprintf("- Current quota state: %s", formatQuotaStatusLine(snapshot)),
		fmt.Sprintf("- Current extra usage state: %s", formatOverageLine(snapshot)),
		"- Aggregated usage history and activity panels remain unmigrated.",
	}
	if strings.TrimSpace(snapshot.Summary) != "" {
		lines = append(lines, fmt.Sprintf("- Probe: %s", snapshot.Summary))
	}
	if persisted, err := claudeailimits.LoadClaudeAILimits(); err == nil {
		if extra := renderClaudeAILimitsLines(persisted); len(extra) > 0 {
			lines = append(lines, extra...)
		}
	}
	return strings.Join(lines, "\n")
}
