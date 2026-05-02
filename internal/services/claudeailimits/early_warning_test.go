package claudeailimits

import (
	"net/http"
	"testing"
)

// withFrozenClock overrides the package-level clock used by computeTimeProgress
// for the duration of one test, then restores it on cleanup.
func withFrozenClock(t *testing.T, fixed float64) {
	t.Helper()
	original := nowSeconds
	nowSeconds = func() float64 { return fixed }
	t.Cleanup(func() {
		nowSeconds = original
	})
}

func TestComputeTimeProgress(t *testing.T) {
	withFrozenClock(t, 1_700_001_800) // 1800s after window start

	// 5-hour window = 18000 seconds. Reset at start + 18000.
	got := computeTimeProgress(1_700_018_000, 5*60*60)
	want := 1800.0 / 18000.0
	if got != want {
		t.Fatalf("computeTimeProgress = %v, want %v", got, want)
	}
}

func TestComputeTimeProgressClampsAboveOne(t *testing.T) {
	withFrozenClock(t, 1_700_100_000)

	// Reset already passed by 50000s but window was only 18000s. Clamps to 1.
	got := computeTimeProgress(1_700_050_000, 18000)
	if got != 1 {
		t.Fatalf("computeTimeProgress = %v, want 1", got)
	}
}

func TestComputeTimeProgressClampsBelowZero(t *testing.T) {
	withFrozenClock(t, 1_700_000_000)

	// Reset is 1_000_000s in the future and window is only 600_000s, so the
	// window hasn't started yet (windowStart > now). Progress clamps to 0.
	got := computeTimeProgress(1_701_000_000, 600_000)
	if got != 0 {
		t.Fatalf("computeTimeProgress = %v, want 0", got)
	}
}

func TestEarlyWarningServerHeader(t *testing.T) {
	withFrozenClock(t, 1_700_000_000)

	headers := http.Header{}
	headers.Set("anthropic-ratelimit-unified-5h-surpassed-threshold", "0.85")
	headers.Set("anthropic-ratelimit-unified-5h-utilization", "0.92")
	headers.Set("anthropic-ratelimit-unified-5h-reset", "1700018000")

	warn := evaluateEarlyWarning(headers, true)
	if warn == nil {
		t.Fatal("expected early warning")
	}
	if warn.Status != QuotaStatusAllowedWarning {
		t.Fatalf("Status = %q, want allowed_warning", warn.Status)
	}
	if warn.RateLimitType != RateLimitFiveHour {
		t.Fatalf("RateLimitType = %q, want five_hour", warn.RateLimitType)
	}
	if !warn.HasSurpassedThreshold || warn.SurpassedThreshold != 0.85 {
		t.Fatalf("SurpassedThreshold = %v (has=%v), want 0.85", warn.SurpassedThreshold, warn.HasSurpassedThreshold)
	}
	if !warn.UnifiedRateLimitFallbackAvailable {
		t.Fatalf("fallback should be true")
	}
}

func TestEarlyWarningTimeRelative5h(t *testing.T) {
	// 5-hour window, reset in 18000s -> windowStart = now -> 0% time progress.
	withFrozenClock(t, 1_700_000_000)

	headers := http.Header{}
	headers.Set("anthropic-ratelimit-unified-5h-utilization", "0.95") // >= 0.9
	headers.Set("anthropic-ratelimit-unified-5h-reset", "1700018000")

	warn := evaluateEarlyWarning(headers, false)
	if warn == nil {
		t.Fatal("expected early warning to fire")
	}
	if warn.RateLimitType != RateLimitFiveHour {
		t.Fatalf("RateLimitType = %q", warn.RateLimitType)
	}
	if warn.Utilization != 0.95 {
		t.Fatalf("Utilization = %v, want 0.95", warn.Utilization)
	}
}

func TestEarlyWarningTimeRelative7dHighUtilization(t *testing.T) {
	withFrozenClock(t, 1_700_000_000)

	// 7-day window, reset in 604800s, time progress = 0%.
	// Utilization 0.8 satisfies the 0.75 threshold gate (timePct cap 0.6).
	headers := http.Header{}
	headers.Set("anthropic-ratelimit-unified-7d-utilization", "0.80")
	headers.Set("anthropic-ratelimit-unified-7d-reset", "1700604800")

	warn := evaluateEarlyWarning(headers, false)
	if warn == nil {
		t.Fatal("expected early warning to fire")
	}
	if warn.RateLimitType != RateLimitSevenDay {
		t.Fatalf("RateLimitType = %q", warn.RateLimitType)
	}
}

func TestEarlyWarningSuppressedAfterTimeWindow(t *testing.T) {
	// 7-day window, time progress = 80% — only the 0.75 threshold (timePct cap 0.6)
	// would fire, but timeProgress 0.8 > 0.6 disables it. Lower thresholds also
	// disabled because their timePct caps are smaller.
	resetsAt := int64(1_700_604_800)
	withFrozenClock(t, float64(resetsAt-int64(0.2*604800)))

	headers := http.Header{}
	headers.Set("anthropic-ratelimit-unified-7d-utilization", "0.80")
	headers.Set("anthropic-ratelimit-unified-7d-reset", "1700604800")

	if warn := evaluateEarlyWarning(headers, false); warn != nil {
		t.Fatalf("unexpected warning: %+v", warn)
	}
}

func TestEarlyWarningNoUtilizationHeader(t *testing.T) {
	headers := http.Header{}
	headers.Set("anthropic-ratelimit-unified-5h-reset", "1700018000")

	if warn := evaluateEarlyWarning(headers, false); warn != nil {
		t.Fatalf("expected nil when utilization header missing")
	}
}

func TestGetEarlyWarningResultPublic(t *testing.T) {
	withFrozenClock(t, 1_700_000_000)

	headers := http.Header{}
	headers.Set("anthropic-ratelimit-unified-7d-surpassed-threshold", "0.5")
	headers.Set("anthropic-ratelimit-unified-7d-utilization", "0.55")
	headers.Set("anthropic-ratelimit-unified-7d-reset", "1700604800")

	if warn := GetEarlyWarningResult(headers, false); warn == nil {
		t.Fatal("expected non-nil warning via public API")
	}
}
