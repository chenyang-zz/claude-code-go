package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const extraUsageCommandFallback = "Extra usage enrollment is not available in Claude Code Go yet. Browser launch, account overage management, and post-enrollment login handoff remain unmigrated."

// ExtraUsageCommand exposes the minimum text-only /extra-usage behavior available before browser and account flows exist in the Go host.
type ExtraUsageCommand struct {
	// Config carries the already-resolved runtime configuration snapshot.
	Config coreconfig.Config
	// Probe performs provider-specific quota checks when available.
	Probe UsageLimitsProber
}

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

	if snapshot, ok := c.snapshot(ctx); ok {
		logger.DebugCF("commands", "rendered extra usage command quota snapshot", map[string]any{
			"provider":                c.Config.Provider,
			"overage_status":          snapshot.OverageStatus,
			"overage_disabled_reason": snapshot.OverageDisabledReason,
		})
		return command.Result{
			Output: renderExtraUsageSnapshot(snapshot),
		}, nil
	}

	logger.DebugCF("commands", "rendered extra-usage command fallback output", map[string]any{
		"browser_launch_supported": false,
		"overage_management":       false,
		"login_handoff_supported":  false,
	})

	return command.Result{
		Output: extraUsageCommandFallback,
	}, nil
}

// snapshot probes the current provider when extra-usage observation is supported in the Go host.
func (c ExtraUsageCommand) snapshot(ctx context.Context) (UsageLimitsSnapshot, bool) {
	if coreconfig.NormalizeProvider(c.Config.Provider) != coreconfig.ProviderAnthropic {
		return UsageLimitsSnapshot{}, false
	}
	if missingUsageCredential(c.Config) || c.Probe == nil {
		return UsageLimitsSnapshot{}, false
	}
	return c.Probe.ProbeUsage(ctx, c.Config), true
}

// renderExtraUsageSnapshot formats one Anthropic extra-usage snapshot for `/extra-usage`.
func renderExtraUsageSnapshot(snapshot UsageLimitsSnapshot) string {
	lines := []string{
		"Anthropic extra usage snapshot:",
		fmt.Sprintf("- Extra usage status: %s", formatOverageLine(snapshot)),
		"- Browser launch, account overage management, and post-enrollment login handoff remain unmigrated.",
	}
	if snapshot.Status != "" {
		lines = append(lines, fmt.Sprintf("- Base quota status: %s", formatQuotaStatusLine(snapshot)))
	}
	if strings.TrimSpace(snapshot.Summary) != "" {
		lines = append(lines, fmt.Sprintf("- Probe: %s", snapshot.Summary))
	}
	return strings.Join(lines, "\n")
}
