package anthropic

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
)

// TestQuotaProbeProbeUsageParsesWarningHeaders verifies the Anthropic quota probe normalizes unified limiter headers.
func TestQuotaProbeProbeUsageParsesWarningHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != quotaProbePath {
			t.Fatalf("request path = %q, want %q", r.URL.Path, quotaProbePath)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("request method = %q, want POST", r.Method)
		}
		w.Header().Set("anthropic-ratelimit-unified-status", "allowed")
		w.Header().Set("anthropic-ratelimit-unified-7d-surpassed-threshold", "0.75")
		w.Header().Set("anthropic-ratelimit-unified-7d-utilization", "0.8")
		w.Header().Set("anthropic-ratelimit-unified-7d-reset", "1760000000")
		w.Header().Set("anthropic-ratelimit-unified-overage-status", "allowed")
		w.Header().Set("anthropic-ratelimit-unified-overage-reset", "1760500000")
		w.Header().Set("anthropic-ratelimit-unified-fallback", "available")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"msg_test","type":"message","role":"assistant","content":[{"type":"text","text":"ok"}]}`))
	}))
	defer server.Close()

	probe := NewQuotaProbe(QuotaProbeConfig{HTTPClient: server.Client()})
	snapshot := probe.ProbeUsage(context.Background(), coreconfig.Config{
		Provider:   coreconfig.ProviderAnthropic,
		APIKey:     "test-key",
		APIBaseURL: server.URL,
		Model:      "claude-sonnet-4-5",
	})

	if !snapshot.Available {
		t.Fatalf("ProbeUsage() available = false, want true")
	}
	if snapshot.Status != "allowed_warning" {
		t.Fatalf("ProbeUsage() status = %q, want allowed_warning", snapshot.Status)
	}
	if snapshot.RateLimitType != "seven_day" {
		t.Fatalf("ProbeUsage() rate limit type = %q, want seven_day", snapshot.RateLimitType)
	}
	if !snapshot.HasUtilization || snapshot.Utilization != 0.8 {
		t.Fatalf("ProbeUsage() utilization = %#v/%v, want 0.8 present", snapshot.Utilization, snapshot.HasUtilization)
	}
	if snapshot.OverageStatus != "allowed" {
		t.Fatalf("ProbeUsage() overage status = %q, want allowed", snapshot.OverageStatus)
	}
	if !snapshot.FallbackAvailable {
		t.Fatalf("ProbeUsage() fallback available = false, want true")
	}
}

// TestQuotaProbeProbeUsageReportsFailure verifies the Anthropic quota probe returns one stable failure summary on transport errors.
func TestQuotaProbeProbeUsageReportsFailure(t *testing.T) {
	probe := NewQuotaProbe(QuotaProbeConfig{HTTPClient: &http.Client{}})
	snapshot := probe.ProbeUsage(context.Background(), coreconfig.Config{
		Provider:   coreconfig.ProviderAnthropic,
		APIKey:     "test-key",
		APIBaseURL: "http://127.0.0.1:1",
	})

	if snapshot.Available {
		t.Fatalf("ProbeUsage() available = true, want false")
	}
	if snapshot.Summary == "" {
		t.Fatal("ProbeUsage() summary = empty, want failure summary")
	}
}
