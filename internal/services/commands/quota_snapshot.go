package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
)

// UsageLimitsSnapshot stores one normalized provider quota summary for slash command rendering.
type UsageLimitsSnapshot struct {
	// Supported reports whether the active provider exposes a migrated quota snapshot.
	Supported bool
	// Available reports whether the probe returned a usable snapshot instead of a transport/build failure.
	Available bool
	// Provider stores the normalized provider identifier that produced this snapshot.
	Provider string
	// Status stores the normalized quota status such as allowed, allowed_warning, or rejected.
	Status string
	// RateLimitType stores the representative quota claim when the provider exposes one.
	RateLimitType string
	// ResetsAt stores the primary quota reset timestamp in unix seconds when available.
	ResetsAt int64
	// Utilization stores the 0-1 utilization fraction when available.
	Utilization float64
	// HasUtilization reports whether Utilization contains a provider value.
	HasUtilization bool
	// OverageStatus stores the provider-side extra-usage status when available.
	OverageStatus string
	// OverageResetsAt stores the extra-usage reset timestamp in unix seconds when available.
	OverageResetsAt int64
	// OverageDisabledReason stores the provider-disabled reason surfaced by Anthropic unified limits.
	OverageDisabledReason string
	// FallbackAvailable reports whether the provider exposed a fallback tier in the unified limiter.
	FallbackAvailable bool
	// Summary stores one stable caller-facing probe status when a live snapshot is unavailable.
	Summary string
}

// UsageLimitsProber defines the minimum provider-side quota probe used by usage-related slash commands.
type UsageLimitsProber interface {
	// ProbeUsage checks whether the configured provider can surface one stable quota snapshot.
	ProbeUsage(ctx context.Context, cfg coreconfig.Config) UsageLimitsSnapshot
}

// missingUsageCredential reports whether the current provider lacks the minimum credential needed for one usage probe.
func missingUsageCredential(cfg coreconfig.Config) bool {
	if coreconfig.NormalizeProvider(cfg.Provider) == coreconfig.ProviderAnthropic {
		return strings.TrimSpace(cfg.APIKey) == "" && strings.TrimSpace(cfg.AuthToken) == ""
	}
	return strings.TrimSpace(cfg.APIKey) == ""
}

// formatQuotaStatusLine converts one normalized quota status into a stable human-readable line body.
func formatQuotaStatusLine(snapshot UsageLimitsSnapshot) string {
	if snapshot.Status == "" {
		return "unknown"
	}

	label := snapshot.Status
	if snapshot.RateLimitType != "" {
		label = fmt.Sprintf("%s (%s)", label, displayRateLimitType(snapshot.RateLimitType))
	}
	if snapshot.ResetsAt > 0 {
		label = fmt.Sprintf("%s; resets %s", label, formatUnixReset(snapshot.ResetsAt))
	}
	if snapshot.HasUtilization {
		label = fmt.Sprintf("%s; utilization %.0f%%", label, snapshot.Utilization*100)
	}
	return label
}

// formatOverageLine converts one normalized extra-usage status into a stable human-readable line body.
func formatOverageLine(snapshot UsageLimitsSnapshot) string {
	if snapshot.OverageStatus == "" {
		if snapshot.OverageDisabledReason != "" {
			return fmt.Sprintf("unavailable (%s)", snapshot.OverageDisabledReason)
		}
		return "not reported"
	}

	label := snapshot.OverageStatus
	if snapshot.OverageDisabledReason != "" {
		label = fmt.Sprintf("%s (%s)", label, snapshot.OverageDisabledReason)
	}
	if snapshot.OverageResetsAt > 0 {
		label = fmt.Sprintf("%s; resets %s", label, formatUnixReset(snapshot.OverageResetsAt))
	}
	return label
}

// displayRateLimitType renders one normalized rate-limit claim into a stable user-facing label.
func displayRateLimitType(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "five_hour":
		return "session limit"
	case "seven_day":
		return "weekly limit"
	case "seven_day_opus":
		return "Opus limit"
	case "seven_day_sonnet":
		return "Sonnet limit"
	case "overage":
		return "extra usage limit"
	default:
		return displayValue(value)
	}
}

// formatUnixReset renders one unix-seconds reset timestamp in stable UTC form.
func formatUnixReset(value int64) string {
	if value <= 0 {
		return "unknown"
	}
	return time.Unix(value, 0).UTC().Format(time.RFC3339)
}
