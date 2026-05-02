package claudeailimits

import (
	"net/http"
	"testing"
)

func TestProcessRateLimitHeadersEmpty(t *testing.T) {
	if got := ProcessRateLimitHeaders(nil); got != nil {
		t.Fatalf("nil headers should return nil, got %+v", got)
	}
	headers := http.Header{}
	if got := ProcessRateLimitHeaders(headers); got != nil {
		t.Fatalf("blank headers should return nil, got %+v", got)
	}
	headers.Set("content-type", "application/json")
	if got := ProcessRateLimitHeaders(headers); got != nil {
		t.Fatalf("non-ratelimit headers should return nil, got %+v", got)
	}
}

func TestProcessRateLimitHeadersAllowed(t *testing.T) {
	withFrozenClock(t, 1_700_000_000)

	headers := http.Header{}
	headers.Set("anthropic-ratelimit-unified-status", "allowed")
	headers.Set("anthropic-ratelimit-unified-fallback", "available")
	headers.Set("anthropic-ratelimit-unified-representative-claim", "five_hour")
	headers.Set("anthropic-ratelimit-unified-reset", "1700050000")
	// Utilization is well below the 0.9 threshold, no early warning fires.
	headers.Set("anthropic-ratelimit-unified-5h-utilization", "0.2")
	headers.Set("anthropic-ratelimit-unified-5h-reset", "1700050000")

	got := ProcessRateLimitHeaders(headers)
	if got == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if got.Status != QuotaStatusAllowed {
		t.Fatalf("Status = %q, want allowed", got.Status)
	}
	if !got.UnifiedRateLimitFallbackAvailable {
		t.Fatalf("UnifiedRateLimitFallbackAvailable should be true")
	}
	if got.RateLimitType != RateLimitFiveHour {
		t.Fatalf("RateLimitType = %q, want five_hour", got.RateLimitType)
	}
	if got.ResetsAt != 1700050000 {
		t.Fatalf("ResetsAt = %d, want 1700050000", got.ResetsAt)
	}
	if !got.HasUtilization || got.Utilization != 0.2 {
		t.Fatalf("Utilization = %v (has=%v), want 0.2", got.Utilization, got.HasUtilization)
	}
	if got.IsUsingOverage {
		t.Fatalf("IsUsingOverage should be false")
	}
}

func TestProcessRateLimitHeadersRejected(t *testing.T) {
	headers := http.Header{}
	headers.Set("anthropic-ratelimit-unified-status", "rejected")
	headers.Set("anthropic-ratelimit-unified-representative-claim", "seven_day")
	headers.Set("anthropic-ratelimit-unified-reset", "1700100000")
	headers.Set("anthropic-ratelimit-unified-overage-status", "allowed")
	headers.Set("anthropic-ratelimit-unified-overage-reset", "1700200000")

	got := ProcessRateLimitHeaders(headers)
	if got == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if got.Status != QuotaStatusRejected {
		t.Fatalf("Status = %q, want rejected", got.Status)
	}
	if got.OverageStatus != QuotaStatusAllowed {
		t.Fatalf("OverageStatus = %q, want allowed", got.OverageStatus)
	}
	if !got.IsUsingOverage {
		t.Fatalf("IsUsingOverage should be true when rejected with allowed overage")
	}
	if got.RateLimitType != RateLimitSevenDay {
		t.Fatalf("RateLimitType = %q, want seven_day", got.RateLimitType)
	}
	if got.OverageResetsAt != 1700200000 {
		t.Fatalf("OverageResetsAt = %d, want 1700200000", got.OverageResetsAt)
	}
}

func TestProcessRateLimitHeadersDisabledOverageReason(t *testing.T) {
	headers := http.Header{}
	headers.Set("anthropic-ratelimit-unified-status", "rejected")
	headers.Set("anthropic-ratelimit-unified-overage-status", "rejected")
	headers.Set("anthropic-ratelimit-unified-overage-disabled-reason", "out_of_credits")

	got := ProcessRateLimitHeaders(headers)
	if got == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if got.OverageDisabledReason != OverageOutOfCredits {
		t.Fatalf("OverageDisabledReason = %q, want out_of_credits", got.OverageDisabledReason)
	}
	if got.IsUsingOverage {
		t.Fatalf("IsUsingOverage should be false when both standard and overage are rejected")
	}
}

func TestExtractRawUtilization(t *testing.T) {
	headers := http.Header{}
	headers.Set("anthropic-ratelimit-unified-5h-utilization", "0.42")
	headers.Set("anthropic-ratelimit-unified-5h-reset", "1700000000")
	// 7d window only has utilization without reset — should be skipped.
	headers.Set("anthropic-ratelimit-unified-7d-utilization", "0.18")

	got := ExtractRawUtilization(headers)
	if got.FiveHour == nil {
		t.Fatal("expected FiveHour to be populated")
	}
	if got.FiveHour.Utilization != 0.42 {
		t.Fatalf("FiveHour.Utilization = %v, want 0.42", got.FiveHour.Utilization)
	}
	if got.FiveHour.ResetsAt != 1700000000 {
		t.Fatalf("FiveHour.ResetsAt = %d, want 1700000000", got.FiveHour.ResetsAt)
	}
	if got.SevenDay != nil {
		t.Fatalf("SevenDay should be nil when reset header is missing")
	}
}
