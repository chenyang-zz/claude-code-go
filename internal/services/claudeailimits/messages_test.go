package claudeailimits

import (
	"strings"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/platform/oauth"
)

// installSubscription is shorthand for setting up a subscription tokens for
// each scenario in messages tests.
func installSubscription(t *testing.T, sub oauth.SubscriptionType, hasExtra bool, billing oauth.BillingType) {
	t.Helper()
	SetSubscriptionLoader(SubscriptionLoaderFunc(func() (*oauth.OAuthTokens, error) {
		return &oauth.OAuthTokens{
			SubscriptionType:     sub,
			HasExtraUsageEnabled: hasExtra,
			BillingType:          billing,
		}, nil
	}))
	t.Cleanup(func() { SetSubscriptionLoader(nil) })
}

func TestIsRateLimitErrorMessage(t *testing.T) {
	t.Run("known_prefixes", func(t *testing.T) {
		for _, prefix := range RateLimitErrorPrefixes {
			text := prefix + " session limit · resets 5pm"
			if !IsRateLimitErrorMessage(text) {
				t.Fatalf("expected prefix %q to match", prefix)
			}
		}
	})
	t.Run("non_matching", func(t *testing.T) {
		if IsRateLimitErrorMessage("Some unrelated error message") {
			t.Fatal("expected false for unrelated text")
		}
	})
}

func TestGetRateLimitMessageNil(t *testing.T) {
	if got := GetRateLimitMessage(nil, "claude-sonnet-4-6"); got != nil {
		t.Fatalf("expected nil for nil limits, got %+v", got)
	}
}

func TestGetRateLimitMessageRejected5h(t *testing.T) {
	installSubscription(t, oauth.SubscriptionPro, false, "")
	limits := &ClaudeAILimits{
		Status:        QuotaStatusRejected,
		RateLimitType: RateLimitFiveHour,
		ResetsAt:      0,
	}
	got := GetRateLimitMessage(limits, "claude-sonnet-4-6")
	if got == nil || got.Severity != SeverityError {
		t.Fatalf("expected error severity, got %+v", got)
	}
	if !strings.HasPrefix(got.Message, "You've hit your session limit") {
		t.Fatalf("unexpected message %q", got.Message)
	}
}

func TestGetRateLimitMessageRejectedSevenDaySonnetProUsesWeekly(t *testing.T) {
	installSubscription(t, oauth.SubscriptionPro, false, "")
	limits := &ClaudeAILimits{
		Status:        QuotaStatusRejected,
		RateLimitType: RateLimitSevenDaySonnet,
	}
	got := GetRateLimitErrorMessage(limits, "claude-sonnet-4-6")
	if !strings.HasPrefix(got, "You've hit your weekly limit") {
		t.Fatalf("expected weekly limit framing for pro, got %q", got)
	}
}

func TestGetRateLimitMessageRejectedSevenDaySonnetTeamUsesSonnet(t *testing.T) {
	installSubscription(t, oauth.SubscriptionTeam, false, "")
	limits := &ClaudeAILimits{
		Status:        QuotaStatusRejected,
		RateLimitType: RateLimitSevenDaySonnet,
	}
	got := GetRateLimitErrorMessage(limits, "claude-sonnet-4-6")
	if !strings.HasPrefix(got, "You've hit your Sonnet limit") {
		t.Fatalf("expected Sonnet framing for team, got %q", got)
	}
}

func TestGetRateLimitMessageBothRejectedOutOfCredits(t *testing.T) {
	installSubscription(t, oauth.SubscriptionMax, true, "")
	limits := &ClaudeAILimits{
		Status:                QuotaStatusRejected,
		OverageStatus:         QuotaStatusRejected,
		OverageDisabledReason: OverageOutOfCredits,
	}
	msg := GetRateLimitErrorMessage(limits, "claude-sonnet-4-6")
	if !strings.HasPrefix(msg, "You're out of extra usage") {
		t.Fatalf("expected out-of-credits message, got %q", msg)
	}
}

func TestGetRateLimitMessageOverageWarning(t *testing.T) {
	installSubscription(t, oauth.SubscriptionMax, true, "")
	limits := &ClaudeAILimits{
		IsUsingOverage: true,
		OverageStatus:  QuotaStatusAllowedWarning,
	}
	got := GetRateLimitMessage(limits, "claude-sonnet-4-6")
	if got == nil || got.Severity != SeverityWarning {
		t.Fatalf("expected warning severity, got %+v", got)
	}
	if got.Message != "You're close to your extra usage spending limit" {
		t.Fatalf("unexpected overage warning message %q", got.Message)
	}
}

func TestGetRateLimitMessageOverageQuiet(t *testing.T) {
	installSubscription(t, oauth.SubscriptionMax, true, "")
	limits := &ClaudeAILimits{
		IsUsingOverage: true,
		OverageStatus:  QuotaStatusAllowed,
	}
	if got := GetRateLimitMessage(limits, "claude-sonnet-4-6"); got != nil {
		t.Fatalf("expected nil during normal overage, got %+v", got)
	}
}

func TestGetRateLimitWarningSuppressedBelowThreshold(t *testing.T) {
	installSubscription(t, oauth.SubscriptionPro, false, "")
	limits := &ClaudeAILimits{
		Status:         QuotaStatusAllowedWarning,
		RateLimitType:  RateLimitFiveHour,
		Utilization:    0.5,
		HasUtilization: true,
	}
	if msg := GetRateLimitWarning(limits, "claude-sonnet-4-6"); msg != "" {
		t.Fatalf("expected suppressed warning, got %q", msg)
	}
}

func TestGetRateLimitWarningProEmitsUpgradeUpsell(t *testing.T) {
	installSubscription(t, oauth.SubscriptionPro, false, "")
	limits := &ClaudeAILimits{
		Status:         QuotaStatusAllowedWarning,
		RateLimitType:  RateLimitFiveHour,
		Utilization:    0.85,
		HasUtilization: true,
	}
	msg := GetRateLimitWarning(limits, "claude-sonnet-4-6")
	if msg == "" {
		t.Fatal("expected warning message")
	}
	if !strings.Contains(msg, "/upgrade to keep using Claude Code") {
		t.Fatalf("expected upgrade upsell, got %q", msg)
	}
}

func TestGetRateLimitWarningTeamWithProvisioningAllowedEmitsExtraUsageUpsell(t *testing.T) {
	installSubscription(t, oauth.SubscriptionTeam, false, "")
	limits := &ClaudeAILimits{
		Status:         QuotaStatusAllowedWarning,
		RateLimitType:  RateLimitFiveHour,
		Utilization:    0.85,
		HasUtilization: true,
	}
	msg := GetRateLimitWarning(limits, "claude-sonnet-4-6")
	if !strings.Contains(msg, "/extra-usage to request more") {
		t.Fatalf("expected extra-usage upsell, got %q", msg)
	}
}

func TestGetRateLimitWarningTeamWithExtraUsageNoBillingAccessIsSilenced(t *testing.T) {
	// Team with overage already enabled and no billing access should be
	// suppressed entirely so users do not see a redundant approaching-limit
	// banner before rolling into overage automatically.
	installSubscription(t, oauth.SubscriptionTeam, true, "")
	limits := &ClaudeAILimits{
		Status:         QuotaStatusAllowedWarning,
		RateLimitType:  RateLimitFiveHour,
		Utilization:    0.9,
		HasUtilization: true,
	}
	if msg := GetRateLimitWarning(limits, "claude-sonnet-4-6"); msg != "" {
		t.Fatalf("expected silenced warning, got %q", msg)
	}
}

func TestGetUsingOverageTextDefaults(t *testing.T) {
	if got := GetUsingOverageText(nil); got != "Now using extra usage" {
		t.Fatalf("expected default fallback, got %q", got)
	}
}

func TestGetUsingOverageTextWithKnownLimit(t *testing.T) {
	installSubscription(t, oauth.SubscriptionPro, true, "")
	limits := &ClaudeAILimits{RateLimitType: RateLimitSevenDay, ResetsAt: 0}
	got := GetUsingOverageText(limits)
	if got != "You're now using extra usage" {
		t.Fatalf("expected truncated message without reset time, got %q", got)
	}
}

func TestGetUsingOverageTextSonnetForPro(t *testing.T) {
	installSubscription(t, oauth.SubscriptionPro, true, "")
	limits := &ClaudeAILimits{RateLimitType: RateLimitSevenDaySonnet}
	got := GetUsingOverageText(limits)
	if got != "You're now using extra usage" {
		t.Fatalf("expected normalized weekly framing, got %q", got)
	}
}

func TestFormatResetTime(t *testing.T) {
	if got := formatResetTime(0); got != "" {
		t.Fatalf("expected empty for zero, got %q", got)
	}
	if got := formatResetTime(-5); got != "" {
		t.Fatalf("expected empty for negative, got %q", got)
	}
}
