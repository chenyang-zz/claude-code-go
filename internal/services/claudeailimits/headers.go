package claudeailimits

import (
	"net/http"
	"strconv"
	"strings"
)

// claimAbbrevForType maps each rate-limit claim onto its Anthropic header
// abbreviation. Used by both header parsing and early-warning fallback.
var claimAbbrevForType = map[RateLimitType]string{
	RateLimitFiveHour: "5h",
	RateLimitSevenDay: "7d",
	RateLimitOverage:  "overage",
}

// earlyWarningClaimMap maps the abbreviated header claim name onto the
// canonical RateLimitType for surpassed-threshold detection.
var earlyWarningClaimMap = map[string]RateLimitType{
	"5h":      RateLimitFiveHour,
	"7d":      RateLimitSevenDay,
	"overage": RateLimitOverage,
}

// ProcessRateLimitHeaders projects one Anthropic response header set onto a
// ClaudeAILimits snapshot. Returns nil when no unified rate-limit fields are
// present, signalling the caller should not overwrite cached state.
func ProcessRateLimitHeaders(headers http.Header) *ClaudeAILimits {
	if headers == nil {
		return nil
	}
	if !hasAnyUnifiedHeader(headers) {
		return nil
	}

	status := QuotaStatus(strings.TrimSpace(headers.Get("anthropic-ratelimit-unified-status")))
	if status == "" {
		status = QuotaStatusAllowed
	}

	resetsAt := headerInt64(headers, "anthropic-ratelimit-unified-reset")
	fallback := strings.EqualFold(strings.TrimSpace(headers.Get("anthropic-ratelimit-unified-fallback")), "available")
	rateLimitType := RateLimitType(strings.TrimSpace(headers.Get("anthropic-ratelimit-unified-representative-claim")))
	overageStatus := QuotaStatus(strings.TrimSpace(headers.Get("anthropic-ratelimit-unified-overage-status")))
	overageResetsAt := headerInt64(headers, "anthropic-ratelimit-unified-overage-reset")
	overageDisabledReason := OverageDisabledReason(strings.TrimSpace(headers.Get("anthropic-ratelimit-unified-overage-disabled-reason")))

	isUsingOverage := status == QuotaStatusRejected &&
		(overageStatus == QuotaStatusAllowed || overageStatus == QuotaStatusAllowedWarning)

	// When the limiter status is allowed or allowed_warning, look for an
	// early-warning signal first (server header) and fall back to the
	// time-relative thresholds. Either match collapses status into
	// allowed_warning and seeds rateLimitType / utilization / resetsAt.
	if status == QuotaStatusAllowed || status == QuotaStatusAllowedWarning {
		if warn := evaluateEarlyWarning(headers, fallback); warn != nil {
			return warn
		}
		// No early warning fired — clear ambiguous allowed_warning back to
		// allowed so callers do not display a phantom warning.
		status = QuotaStatusAllowed
	}

	limits := &ClaudeAILimits{
		Status:                            status,
		UnifiedRateLimitFallbackAvailable: fallback,
		ResetsAt:                          resetsAt,
		RateLimitType:                     rateLimitType,
		OverageStatus:                     overageStatus,
		OverageResetsAt:                   overageResetsAt,
		OverageDisabledReason:             overageDisabledReason,
		IsUsingOverage:                    isUsingOverage,
	}

	// Pull the representative claim utilization onto the snapshot when we can.
	if abbrev, ok := claimAbbrevForType[rateLimitType]; ok {
		if util, present := headerFloat(headers, "anthropic-ratelimit-unified-"+abbrev+"-utilization"); present {
			limits.Utilization = util
			limits.HasUtilization = true
		}
	}

	return limits
}

// ExtractRawUtilization projects the per-window utilization headers onto a
// RawUtilization snapshot. Only fields that have both utilization and reset
// headers populate the resulting struct.
func ExtractRawUtilization(headers http.Header) RawUtilization {
	result := RawUtilization{}
	if headers == nil {
		return result
	}

	for _, claim := range []struct {
		abbrev string
		assign func(*RawWindowUtilization)
	}{
		{
			abbrev: "5h",
			assign: func(w *RawWindowUtilization) { result.FiveHour = w },
		},
		{
			abbrev: "7d",
			assign: func(w *RawWindowUtilization) { result.SevenDay = w },
		},
	} {
		util, hasUtil := headerFloat(headers, "anthropic-ratelimit-unified-"+claim.abbrev+"-utilization")
		reset := headerInt64(headers, "anthropic-ratelimit-unified-"+claim.abbrev+"-reset")
		if !hasUtil || reset == 0 {
			continue
		}
		claim.assign(&RawWindowUtilization{Utilization: util, ResetsAt: reset})
	}
	return result
}

// hasAnyUnifiedHeader reports whether the response carries at least one
// unified rate-limit field. Used to short-circuit when the upstream response
// is not from a Claude.ai-aware endpoint.
func hasAnyUnifiedHeader(headers http.Header) bool {
	probes := []string{
		"anthropic-ratelimit-unified-status",
		"anthropic-ratelimit-unified-reset",
		"anthropic-ratelimit-unified-fallback",
		"anthropic-ratelimit-unified-representative-claim",
		"anthropic-ratelimit-unified-overage-status",
		"anthropic-ratelimit-unified-5h-utilization",
		"anthropic-ratelimit-unified-7d-utilization",
		"anthropic-ratelimit-unified-overage-utilization",
		"anthropic-ratelimit-unified-5h-surpassed-threshold",
		"anthropic-ratelimit-unified-7d-surpassed-threshold",
		"anthropic-ratelimit-unified-overage-surpassed-threshold",
	}
	for _, key := range probes {
		if strings.TrimSpace(headers.Get(key)) != "" {
			return true
		}
	}
	return false
}

// headerInt64 parses the named integer header (unix-seconds form) tolerating
// blank or invalid input.
func headerInt64(headers http.Header, key string) int64 {
	value := strings.TrimSpace(headers.Get(key))
	if value == "" {
		return 0
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0
	}
	return parsed
}

// headerFloat parses the named float header (0-1 utilization fraction).
// The second return reports whether the field was present so callers can
// distinguish "no header" from "header == 0".
func headerFloat(headers http.Header, key string) (float64, bool) {
	value := strings.TrimSpace(headers.Get(key))
	if value == "" {
		return 0, false
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, false
	}
	return parsed, true
}
