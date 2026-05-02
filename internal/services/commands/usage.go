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

const usageCommandFallback = "Plan usage limits UI is not available in Claude Code Go yet. Settings Usage tab rendering, account plan limit lookup, and consumer subscription state detection remain unmigrated."

// UsageCommand exposes the minimum text-only /usage behavior available before the settings usage view exists in the Go host.
type UsageCommand struct {
	// Config carries the already-resolved runtime configuration snapshot.
	Config coreconfig.Config
	// Probe performs provider-specific quota checks when available.
	Probe UsageLimitsProber
}

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

	if snapshot, ok := c.snapshot(ctx); ok {
		logger.DebugCF("commands", "rendered usage command quota snapshot", map[string]any{
			"provider":        c.Config.Provider,
			"quota_status":    snapshot.Status,
			"overage_status":  snapshot.OverageStatus,
			"fallback_status": snapshot.FallbackAvailable,
		})
		return command.Result{
			Output: renderUsageSnapshot(snapshot),
		}, nil
	}

	logger.DebugCF("commands", "rendered usage command fallback output", map[string]any{
		"usage_limits_ui_available": false,
		"plan_limit_lookup":         false,
	})

	return command.Result{
		Output: usageCommandFallback,
	}, nil
}

// snapshot probes the current provider when usage observation is supported in the Go host.
func (c UsageCommand) snapshot(ctx context.Context) (UsageLimitsSnapshot, bool) {
	if coreconfig.NormalizeProvider(c.Config.Provider) != coreconfig.ProviderAnthropic {
		return UsageLimitsSnapshot{}, false
	}
	if missingUsageCredential(c.Config) || c.Probe == nil {
		return UsageLimitsSnapshot{}, false
	}
	return c.Probe.ProbeUsage(ctx, c.Config), true
}

// renderUsageSnapshot formats one Anthropic quota snapshot for `/usage`.
func renderUsageSnapshot(snapshot UsageLimitsSnapshot) string {
	lines := []string{
		"Anthropic usage snapshot:",
		fmt.Sprintf("- Status: %s", formatQuotaStatusLine(snapshot)),
		fmt.Sprintf("- Extra usage: %s", formatOverageLine(snapshot)),
		fmt.Sprintf("- Fallback tier: %s", renderAvailability(snapshot.FallbackAvailable)),
		"- Settings Usage tab, account plan lookup, and historical usage charts remain unmigrated.",
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

// renderAvailability converts one boolean into a stable yes/no label.
func renderAvailability(enabled bool) string {
	if enabled {
		return "available"
	}
	return "not reported"
}
