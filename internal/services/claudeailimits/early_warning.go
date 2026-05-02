package claudeailimits

import (
	"net/http"
	"time"
)

// EarlyWarningConfigs enumerates the time-relative early warning rules used
// when the server does not push a `surpassed-threshold` header. Evaluated in
// declaration order — the first match wins.
var EarlyWarningConfigs = []EarlyWarningConfig{
	{
		RateLimitType: RateLimitFiveHour,
		ClaimAbbrev:   "5h",
		WindowSeconds: 5 * 60 * 60,
		Thresholds: []EarlyWarningThreshold{
			{Utilization: 0.9, TimePct: 0.72},
		},
	},
	{
		RateLimitType: RateLimitSevenDay,
		ClaimAbbrev:   "7d",
		WindowSeconds: 7 * 24 * 60 * 60,
		Thresholds: []EarlyWarningThreshold{
			{Utilization: 0.75, TimePct: 0.6},
			{Utilization: 0.5, TimePct: 0.35},
			{Utilization: 0.25, TimePct: 0.15},
		},
	},
}

// nowSeconds is overridable for tests to inject a deterministic clock.
var nowSeconds = func() float64 {
	return float64(time.Now().UnixNano()) / float64(time.Second)
}

// computeTimeProgress returns the 0-1 fraction of the window that has elapsed
// before the supplied reset timestamp. Used for time-relative early warning.
func computeTimeProgress(resetsAt int64, windowSeconds int64) float64 {
	if windowSeconds <= 0 {
		return 0
	}
	now := nowSeconds()
	windowStart := float64(resetsAt) - float64(windowSeconds)
	elapsed := now - windowStart
	if elapsed < 0 {
		return 0
	}
	progress := elapsed / float64(windowSeconds)
	if progress > 1 {
		return 1
	}
	return progress
}

// evaluateEarlyWarning chooses between the server-side surpassed-threshold
// header and the time-relative fallback. Returns nil when no warning fired.
func evaluateEarlyWarning(headers http.Header, fallbackAvailable bool) *ClaudeAILimits {
	if headers == nil {
		return nil
	}
	if warn := getHeaderBasedEarlyWarning(headers, fallbackAvailable); warn != nil {
		return warn
	}
	for _, cfg := range EarlyWarningConfigs {
		if warn := getTimeRelativeEarlyWarning(headers, cfg, fallbackAvailable); warn != nil {
			return warn
		}
	}
	return nil
}

// getHeaderBasedEarlyWarning checks each claim for a surpassed-threshold
// header pushed by the server. Returns the first match as a fully populated
// ClaudeAILimits snapshot.
func getHeaderBasedEarlyWarning(headers http.Header, fallbackAvailable bool) *ClaudeAILimits {
	for abbrev, rateLimitType := range earlyWarningClaimMap {
		key := "anthropic-ratelimit-unified-" + abbrev + "-surpassed-threshold"
		thresholdStr := headers.Get(key)
		if thresholdStr == "" {
			continue
		}
		threshold, present := headerFloat(headers, key)
		if !present {
			continue
		}

		util, hasUtil := headerFloat(headers, "anthropic-ratelimit-unified-"+abbrev+"-utilization")
		resetsAt := headerInt64(headers, "anthropic-ratelimit-unified-"+abbrev+"-reset")

		warn := &ClaudeAILimits{
			Status:                            QuotaStatusAllowedWarning,
			UnifiedRateLimitFallbackAvailable: fallbackAvailable,
			RateLimitType:                     rateLimitType,
			ResetsAt:                          resetsAt,
			IsUsingOverage:                    false,
			SurpassedThreshold:                threshold,
			HasSurpassedThreshold:             true,
		}
		if hasUtil {
			warn.Utilization = util
			warn.HasUtilization = true
		}
		return warn
	}
	return nil
}

// getTimeRelativeEarlyWarning evaluates the time-relative thresholds for one
// claim configuration. Fires when both utilization and time-progress satisfy
// any threshold pair. Returns nil otherwise.
func getTimeRelativeEarlyWarning(headers http.Header, cfg EarlyWarningConfig, fallbackAvailable bool) *ClaudeAILimits {
	utilKey := "anthropic-ratelimit-unified-" + cfg.ClaimAbbrev + "-utilization"
	resetKey := "anthropic-ratelimit-unified-" + cfg.ClaimAbbrev + "-reset"

	utilization, hasUtil := headerFloat(headers, utilKey)
	if !hasUtil {
		return nil
	}
	resetsAt := headerInt64(headers, resetKey)
	if resetsAt == 0 {
		return nil
	}

	timeProgress := computeTimeProgress(resetsAt, cfg.WindowSeconds)
	for _, t := range cfg.Thresholds {
		if utilization >= t.Utilization && timeProgress <= t.TimePct {
			return &ClaudeAILimits{
				Status:                            QuotaStatusAllowedWarning,
				UnifiedRateLimitFallbackAvailable: fallbackAvailable,
				ResetsAt:                          resetsAt,
				RateLimitType:                     cfg.RateLimitType,
				Utilization:                       utilization,
				HasUtilization:                    true,
				IsUsingOverage:                    false,
			}
		}
	}
	return nil
}

// GetEarlyWarningResult exposes the early-warning evaluation as a public API
// for callers that want to inspect headers without going through the full
// ProcessRateLimitHeaders projection.
func GetEarlyWarningResult(headers http.Header, fallbackAvailable bool) *ClaudeAILimits {
	return evaluateEarlyWarning(headers, fallbackAvailable)
}
