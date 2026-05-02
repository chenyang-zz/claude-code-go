package claudeailimits

// QuotaStatus represents the high-level limiter decision returned by the
// Anthropic unified rate-limit headers.
type QuotaStatus string

const (
	// QuotaStatusAllowed indicates the request was permitted with no warning.
	QuotaStatusAllowed QuotaStatus = "allowed"
	// QuotaStatusAllowedWarning indicates the request was permitted but the
	// caller is approaching a limit.
	QuotaStatusAllowedWarning QuotaStatus = "allowed_warning"
	// QuotaStatusRejected indicates the request was rejected because a limit
	// was reached.
	QuotaStatusRejected QuotaStatus = "rejected"
)

// RateLimitType enumerates the canonical rate-limit claim names returned by
// the Anthropic unified limiter.
type RateLimitType string

const (
	// RateLimitFiveHour is the rolling 5-hour session window.
	RateLimitFiveHour RateLimitType = "five_hour"
	// RateLimitSevenDay is the rolling 7-day weekly window.
	RateLimitSevenDay RateLimitType = "seven_day"
	// RateLimitSevenDayOpus is the rolling 7-day Opus-specific window.
	RateLimitSevenDayOpus RateLimitType = "seven_day_opus"
	// RateLimitSevenDaySonnet is the rolling 7-day Sonnet-specific window.
	RateLimitSevenDaySonnet RateLimitType = "seven_day_sonnet"
	// RateLimitOverage is the overage / extra-usage tier window.
	RateLimitOverage RateLimitType = "overage"
)

// OverageDisabledReason enumerates the structured reasons surfaced by the
// Anthropic unified limiter when the overage tier is disabled or rejected.
// Stored as an open string so future server-side reasons flow through.
type OverageDisabledReason string

const (
	// OverageNotProvisioned signals overage is not provisioned for this
	// org or seat tier.
	OverageNotProvisioned OverageDisabledReason = "overage_not_provisioned"
	// OverageOrgLevelDisabled signals the organization disabled overage.
	OverageOrgLevelDisabled OverageDisabledReason = "org_level_disabled"
	// OverageOrgLevelDisabledUntil signals the organization temporarily
	// disabled overage.
	OverageOrgLevelDisabledUntil OverageDisabledReason = "org_level_disabled_until"
	// OverageOutOfCredits signals the organization has insufficient credits.
	OverageOutOfCredits OverageDisabledReason = "out_of_credits"
	// OverageSeatTierLevelDisabled signals the seat tier disabled overage.
	OverageSeatTierLevelDisabled OverageDisabledReason = "seat_tier_level_disabled"
	// OverageMemberLevelDisabled signals the member account disabled overage.
	OverageMemberLevelDisabled OverageDisabledReason = "member_level_disabled"
	// OverageSeatTierZeroCreditLimit signals the seat tier has a zero credit
	// limit.
	OverageSeatTierZeroCreditLimit OverageDisabledReason = "seat_tier_zero_credit_limit"
	// OverageGroupZeroCreditLimit signals the resolved group has a zero
	// credit limit.
	OverageGroupZeroCreditLimit OverageDisabledReason = "group_zero_credit_limit"
	// OverageMemberZeroCreditLimit signals the member account has a zero
	// credit limit.
	OverageMemberZeroCreditLimit OverageDisabledReason = "member_zero_credit_limit"
	// OverageOrgServiceLevelDisabled signals the org service disabled overage.
	OverageOrgServiceLevelDisabled OverageDisabledReason = "org_service_level_disabled"
	// OverageOrgServiceZeroCreditLimit signals the org service has a zero
	// credit limit.
	OverageOrgServiceZeroCreditLimit OverageDisabledReason = "org_service_zero_credit_limit"
	// OverageNoLimitsConfigured signals no overage limits are configured for
	// the account.
	OverageNoLimitsConfigured OverageDisabledReason = "no_limits_configured"
	// OverageUnknown is the catch-all for unrecognised reasons.
	OverageUnknown OverageDisabledReason = "unknown"
)

// EarlyWarningThreshold pairs one utilization fraction with one elapsed
// time-progress fraction. The warning fires when both bounds are satisfied:
// utilization is at or above the floor and the time window has not progressed
// beyond the cap.
type EarlyWarningThreshold struct {
	// Utilization is the 0-1 fraction at or above which the warning becomes
	// eligible.
	Utilization float64
	// TimePct is the 0-1 fraction of the window that may have elapsed before
	// the warning becomes ineligible.
	TimePct float64
}

// EarlyWarningConfig captures the time-relative early warning configuration
// for one rate-limit claim. Used as a fallback when the server does not push
// a `surpassed-threshold` header.
type EarlyWarningConfig struct {
	// RateLimitType identifies which canonical claim this config applies to.
	RateLimitType RateLimitType
	// ClaimAbbrev is the abbreviation used in the Anthropic header keys
	// (e.g. "5h", "7d").
	ClaimAbbrev string
	// WindowSeconds is the duration of the rolling window in seconds.
	WindowSeconds int64
	// Thresholds enumerate the (utilization, timePct) gates that trigger a
	// warning. Evaluated in order; the first match wins.
	Thresholds []EarlyWarningThreshold
}

// ClaudeAILimits captures the projected state of the Anthropic unified
// limiter for the active Claude.ai subscription.
type ClaudeAILimits struct {
	// Status records the high-level limiter decision.
	Status QuotaStatus
	// UnifiedRateLimitFallbackAvailable records whether the limiter exposed
	// a fallback tier that allows degraded service. Used to warn Opus users
	// before they exhaust quota.
	UnifiedRateLimitFallbackAvailable bool
	// ResetsAt is the unix epoch (seconds) at which the primary window resets.
	// Zero indicates the field is not present.
	ResetsAt int64
	// RateLimitType is the canonical claim that drove the current decision.
	RateLimitType RateLimitType
	// Utilization is the 0-1 fraction of the active window currently consumed.
	// Negative values indicate the field is not present.
	Utilization float64
	// HasUtilization records whether Utilization carries a server value.
	HasUtilization bool
	// OverageStatus records the limiter decision for the overage tier.
	OverageStatus QuotaStatus
	// OverageResetsAt is the unix epoch (seconds) at which the overage window
	// resets. Zero indicates the field is not present.
	OverageResetsAt int64
	// OverageDisabledReason captures why the overage tier is disabled, when
	// available.
	OverageDisabledReason OverageDisabledReason
	// IsUsingOverage records whether the request was served by the overage
	// tier (standard limits rejected, overage allowed).
	IsUsingOverage bool
	// SurpassedThreshold captures the threshold value pushed by the server
	// when it generated an early-warning header. Zero with HasSurpassedThreshold
	// false means the field is not present.
	SurpassedThreshold        float64
	HasSurpassedThreshold     bool
	// CachedExtraUsageDisabledReason is the persisted snapshot of the
	// overage disabled reason, mirrored to global settings so subsequent
	// sessions can render the same fallback reason.
	CachedExtraUsageDisabledReason OverageDisabledReason
}

// RawWindowUtilization captures the per-window utilization snapshot exposed
// by the Anthropic limiter. Used by statusline scripts to render real-time
// usage without hooking the full ClaudeAILimits state.
type RawWindowUtilization struct {
	// Utilization is the 0-1 fraction of the active window currently consumed.
	Utilization float64
	// ResetsAt is the unix epoch (seconds) at which the window resets.
	ResetsAt int64
}

// RawUtilization is the projection of the raw 5h / 7d window utilization
// snapshots returned by the Anthropic limiter.
type RawUtilization struct {
	// FiveHour is the snapshot of the rolling 5-hour window, when present.
	FiveHour *RawWindowUtilization
	// SevenDay is the snapshot of the rolling 7-day window, when present.
	SevenDay *RawWindowUtilization
}

// DefaultLimits returns the baseline limits used when no Anthropic response
// has been observed yet.
func DefaultLimits() ClaudeAILimits {
	return ClaudeAILimits{
		Status:                            QuotaStatusAllowed,
		UnifiedRateLimitFallbackAvailable: false,
		IsUsingOverage:                    false,
	}
}

// DisplayName projects one rate-limit claim onto the user-facing label used
// by warning and error messages.
func (t RateLimitType) DisplayName() string {
	switch t {
	case RateLimitFiveHour:
		return "session limit"
	case RateLimitSevenDay:
		return "weekly limit"
	case RateLimitSevenDayOpus:
		return "Opus limit"
	case RateLimitSevenDaySonnet:
		return "Sonnet limit"
	case RateLimitOverage:
		return "extra usage limit"
	default:
		if t == "" {
			return ""
		}
		return string(t)
	}
}

// IsKnown reports whether the rate-limit claim matches one of the canonical
// values. Unknown values flow through verbatim but callers may want to gate
// behaviour on this check.
func (t RateLimitType) IsKnown() bool {
	switch t {
	case RateLimitFiveHour, RateLimitSevenDay, RateLimitSevenDayOpus,
		RateLimitSevenDaySonnet, RateLimitOverage:
		return true
	}
	return false
}
