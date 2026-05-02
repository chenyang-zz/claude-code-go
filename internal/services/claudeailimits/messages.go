package claudeailimits

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/platform/oauth"
)

// RateLimitErrorPrefixes enumerates every prefix the rate-limit message
// generator can produce. Exposed so UI components can do prefix matching
// without hard-coding string literals.
var RateLimitErrorPrefixes = []string{
	"You've hit your",
	"You've used",
	"You're now using extra usage",
	"You're close to",
	"You're out of extra usage",
}

// MessageSeverity classifies the rendered rate-limit message.
type MessageSeverity string

const (
	// SeverityError is used for hard-rejection messages displayed inline in
	// the assistant transcript.
	SeverityError MessageSeverity = "error"
	// SeverityWarning is used for upcoming-limit hints displayed in the
	// status footer.
	SeverityWarning MessageSeverity = "warning"
)

// RateLimitMessage pairs the rendered text with its severity classification.
type RateLimitMessage struct {
	Message  string
	Severity MessageSeverity
}

// warningSuppressionThreshold is the lower utilization bound below which
// warning generation is suppressed. Mirrors the TS-side WARNING_THRESHOLD
// (0.7) and prevents stale `allowed_warning` headers from triggering
// spurious banners after a window resets.
const warningSuppressionThreshold = 0.7

// feedbackChannelAnt is the Slack channel mentioned in the Ant-employee
// flavour of the limit-reached message. Mirrors FEEDBACK_CHANNEL_ANT in
// the TS source.
const feedbackChannelAnt = "#briarpatch-cc"

// IsRateLimitErrorMessage reports whether the supplied text begins with one
// of the rate-limit message prefixes. Exposed so UI layers can detect
// rate-limit surfaces without coupling to internals.
func IsRateLimitErrorMessage(text string) bool {
	for _, prefix := range RateLimitErrorPrefixes {
		if strings.HasPrefix(text, prefix) {
			return true
		}
	}
	return false
}

// GetRateLimitMessage returns the appropriate rendered message for the
// supplied limits snapshot, or nil when no message should be shown. The
// model argument is reserved for future per-model branching; the current
// implementation does not vary by model.
func GetRateLimitMessage(limits *ClaudeAILimits, model string) *RateLimitMessage {
	if limits == nil {
		return nil
	}

	// Overage flow first: when standard limits rejected but overage active,
	// only render a warning if the overage tier itself is approaching its
	// cap. The transient "Now using extra usage" banner is rendered via
	// GetUsingOverageText.
	if limits.IsUsingOverage {
		if limits.OverageStatus == QuotaStatusAllowedWarning {
			return &RateLimitMessage{
				Message:  "You're close to your extra usage spending limit",
				Severity: SeverityWarning,
			}
		}
		return nil
	}

	if limits.Status == QuotaStatusRejected {
		return &RateLimitMessage{
			Message:  getLimitReachedText(limits, model),
			Severity: SeverityError,
		}
	}

	if limits.Status == QuotaStatusAllowedWarning {
		// Suppress warnings when utilization is below the suppression
		// threshold to avoid stale-data flicker after a window reset.
		if limits.HasUtilization && limits.Utilization < warningSuppressionThreshold {
			return nil
		}
		// Don't warn non-billing team / enterprise users about approaching
		// plan limits if overages are enabled — they'll seamlessly roll
		// into overage on the next request.
		subscription := GetSubscriptionType()
		isTeamOrEnterprise := subscription == oauth.SubscriptionTeam || subscription == oauth.SubscriptionEnterprise
		if isTeamOrEnterprise && HasExtraUsageEnabled() && !HasClaudeAIBillingAccess() {
			return nil
		}
		text := getEarlyWarningText(limits)
		if text == "" {
			return nil
		}
		return &RateLimitMessage{Message: text, Severity: SeverityWarning}
	}

	return nil
}

// GetRateLimitErrorMessage returns the user-facing error message for an API
// rejection, or empty string when no error message should be surfaced. Used
// by the engine error path to replace generic 429 strings.
func GetRateLimitErrorMessage(limits *ClaudeAILimits, model string) string {
	msg := GetRateLimitMessage(limits, model)
	if msg == nil || msg.Severity != SeverityError {
		return ""
	}
	return msg.Message
}

// GetRateLimitWarning returns the user-facing warning text for the status
// footer, or empty string when no warning should be surfaced.
func GetRateLimitWarning(limits *ClaudeAILimits, model string) string {
	msg := GetRateLimitMessage(limits, model)
	if msg == nil || msg.Severity != SeverityWarning {
		return ""
	}
	return msg.Message
}

// GetUsingOverageText renders the transient "now using extra usage" banner.
// Returns a stable fallback when the rate-limit type is unknown so callers
// always have something to display.
func GetUsingOverageText(limits *ClaudeAILimits) string {
	if limits == nil {
		return "Now using extra usage"
	}

	resetText := formatResetTime(limits.ResetsAt)

	limitName := ""
	switch limits.RateLimitType {
	case RateLimitFiveHour:
		limitName = "session limit"
	case RateLimitSevenDay:
		limitName = "weekly limit"
	case RateLimitSevenDayOpus:
		limitName = "Opus limit"
	case RateLimitSevenDaySonnet:
		// For pro / enterprise, Sonnet usage rolls up into the weekly limit.
		subscription := GetSubscriptionType()
		if subscription == oauth.SubscriptionPro || subscription == oauth.SubscriptionEnterprise {
			limitName = "weekly limit"
		} else {
			limitName = "Sonnet limit"
		}
	}

	if limitName == "" {
		return "Now using extra usage"
	}

	if resetText == "" {
		return "You're now using extra usage"
	}
	return fmt.Sprintf("You're now using extra usage · Your %s resets %s", limitName, resetText)
}

// getLimitReachedText renders the hard-rejection message used when limits
// are exhausted. Ordering matches the TS source so the most specific branch
// wins.
func getLimitReachedText(limits *ClaudeAILimits, model string) string {
	resetText := formatResetTime(limits.ResetsAt)
	overageResetText := formatResetTime(limits.OverageResetsAt)
	resetMessage := ""
	if resetText != "" {
		resetMessage = " · resets " + resetText
	}

	// Both subscription and overage exhausted — combine reset times.
	if limits.OverageStatus == QuotaStatusRejected {
		overageResetMessage := ""
		switch {
		case limits.ResetsAt > 0 && limits.OverageResetsAt > 0:
			if limits.ResetsAt < limits.OverageResetsAt {
				overageResetMessage = " · resets " + resetText
			} else {
				overageResetMessage = " · resets " + overageResetText
			}
		case resetText != "":
			overageResetMessage = " · resets " + resetText
		case overageResetText != "":
			overageResetMessage = " · resets " + overageResetText
		}

		if limits.OverageDisabledReason == OverageOutOfCredits {
			return "You're out of extra usage" + overageResetMessage
		}
		return formatLimitReachedText("limit", overageResetMessage, model)
	}

	switch limits.RateLimitType {
	case RateLimitSevenDaySonnet:
		subscription := GetSubscriptionType()
		isProOrEnterprise := subscription == oauth.SubscriptionPro || subscription == oauth.SubscriptionEnterprise
		if isProOrEnterprise {
			return formatLimitReachedText("weekly limit", resetMessage, model)
		}
		return formatLimitReachedText("Sonnet limit", resetMessage, model)
	case RateLimitSevenDayOpus:
		return formatLimitReachedText("Opus limit", resetMessage, model)
	case RateLimitSevenDay:
		return formatLimitReachedText("weekly limit", resetMessage, model)
	case RateLimitFiveHour:
		return formatLimitReachedText("session limit", resetMessage, model)
	}
	return formatLimitReachedText("usage limit", resetMessage, model)
}

// getEarlyWarningText renders the approaching-limit hint shown in the status
// footer. Returns an empty string when the snapshot does not have enough
// data to construct a meaningful warning.
func getEarlyWarningText(limits *ClaudeAILimits) string {
	limitName := ""
	switch limits.RateLimitType {
	case RateLimitSevenDay:
		limitName = "weekly limit"
	case RateLimitFiveHour:
		limitName = "session limit"
	case RateLimitSevenDayOpus:
		limitName = "Opus limit"
	case RateLimitSevenDaySonnet:
		limitName = "Sonnet limit"
	case RateLimitOverage:
		limitName = "extra usage"
	}
	if limitName == "" {
		return ""
	}

	usedPct := 0
	if limits.HasUtilization {
		usedPct = int(limits.Utilization * 100)
	}
	resetText := formatResetTime(limits.ResetsAt)

	upsell := getWarningUpsellText(limits.RateLimitType)

	switch {
	case usedPct > 0 && resetText != "":
		base := fmt.Sprintf("You've used %d%% of your %s · resets %s", usedPct, limitName, resetText)
		return appendUpsell(base, upsell)
	case usedPct > 0:
		base := fmt.Sprintf("You've used %d%% of your %s", usedPct, limitName)
		return appendUpsell(base, upsell)
	}

	if limits.RateLimitType == RateLimitOverage {
		// `Approaching extra usage` reads better with the trailing word
		// when overages are involved; mirrors the TS-side adjustment.
		limitName += " limit"
	}

	if resetText != "" {
		base := fmt.Sprintf("Approaching %s · resets %s", limitName, resetText)
		return appendUpsell(base, upsell)
	}
	return appendUpsell("Approaching "+limitName, upsell)
}

// getWarningUpsellText returns the appended upsell hint for the supplied
// rate-limit type, or empty string when no upsell applies. Mirrors the
// branching logic on the TS side.
func getWarningUpsellText(rateLimitType RateLimitType) string {
	subscription := GetSubscriptionType()
	hasExtra := HasExtraUsageEnabled()

	switch rateLimitType {
	case RateLimitFiveHour:
		switch subscription {
		case oauth.SubscriptionTeam, oauth.SubscriptionEnterprise:
			if !hasExtra && IsOverageProvisioningAllowed() {
				return "/extra-usage to request more"
			}
			return ""
		case oauth.SubscriptionPro, oauth.SubscriptionMax:
			return "/upgrade to keep using Claude Code"
		}
	case RateLimitOverage:
		switch subscription {
		case oauth.SubscriptionTeam, oauth.SubscriptionEnterprise:
			if !hasExtra && IsOverageProvisioningAllowed() {
				return "/extra-usage to request more"
			}
		}
	}
	return ""
}

// appendUpsell concatenates an upsell suffix when present.
func appendUpsell(base, upsell string) string {
	if upsell == "" {
		return base
	}
	return base + " · " + upsell
}

// formatLimitReachedText renders the hard-rejection sentence for the named
// limit. The function honours the USER_TYPE=ant override so internal users
// see a Slack feedback hint and the /reset-limits affordance.
func formatLimitReachedText(limit, resetMessage, _model string) string {
	if os.Getenv("USER_TYPE") == "ant" {
		return fmt.Sprintf(
			"You've hit your %s%s. If you have feedback about this limit, post in %s. You can reset your limits with /reset-limits",
			limit, resetMessage, feedbackChannelAnt,
		)
	}
	return fmt.Sprintf("You've hit your %s%s", limit, resetMessage)
}

// formatResetTime renders one unix-seconds timestamp as a short, lower-case
// human-friendly string. Returns empty when the timestamp is zero.
//
// The Go implementation is intentionally simpler than the TS reference: we
// always include the time and only include the date when the reset is more
// than 24 hours away. Local timezone is used so the rendered text matches
// the user's wall clock.
func formatResetTime(unixSeconds int64) string {
	if unixSeconds <= 0 {
		return ""
	}
	target := time.Unix(unixSeconds, 0)
	hoursUntil := time.Until(target).Hours()
	clock := target.Format("3:04pm")
	if target.Minute() == 0 {
		clock = target.Format("3pm")
	}
	if hoursUntil > 24 {
		datePart := target.Format("Jan 2")
		if target.Year() != time.Now().Year() {
			datePart = target.Format("Jan 2 2006")
		}
		return fmt.Sprintf("%s, %s", datePart, clock)
	}
	return clock
}
