package claudeailimits

import "testing"

func TestRateLimitTypeDisplayName(t *testing.T) {
	cases := []struct {
		name string
		in   RateLimitType
		want string
	}{
		{name: "five_hour", in: RateLimitFiveHour, want: "session limit"},
		{name: "seven_day", in: RateLimitSevenDay, want: "weekly limit"},
		{name: "seven_day_opus", in: RateLimitSevenDayOpus, want: "Opus limit"},
		{name: "seven_day_sonnet", in: RateLimitSevenDaySonnet, want: "Sonnet limit"},
		{name: "overage", in: RateLimitOverage, want: "extra usage limit"},
		{name: "empty", in: "", want: ""},
		{name: "unknown_passthrough", in: "future_tier", want: "future_tier"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.in.DisplayName(); got != tc.want {
				t.Fatalf("DisplayName(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestRateLimitTypeIsKnown(t *testing.T) {
	known := []RateLimitType{
		RateLimitFiveHour,
		RateLimitSevenDay,
		RateLimitSevenDayOpus,
		RateLimitSevenDaySonnet,
		RateLimitOverage,
	}
	for _, tier := range known {
		if !tier.IsKnown() {
			t.Fatalf("expected %q to be known", tier)
		}
	}
	unknown := []RateLimitType{"", "future_tier", "rolling_30_day"}
	for _, tier := range unknown {
		if tier.IsKnown() {
			t.Fatalf("expected %q to be unknown", tier)
		}
	}
}

func TestDefaultLimits(t *testing.T) {
	got := DefaultLimits()
	if got.Status != QuotaStatusAllowed {
		t.Fatalf("Status = %q, want allowed", got.Status)
	}
	if got.UnifiedRateLimitFallbackAvailable {
		t.Fatalf("UnifiedRateLimitFallbackAvailable should default to false")
	}
	if got.IsUsingOverage {
		t.Fatalf("IsUsingOverage should default to false")
	}
}
