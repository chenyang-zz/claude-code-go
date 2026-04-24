package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
)

// TestQuotaProbeUsageSuccess verifies quota probe parses rate-limit headers from a successful response.
func TestQuotaProbeUsageSuccess(t *testing.T) {
	resetTime := time.Now().Add(5 * time.Minute).UTC()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %q, want POST", r.Method)
		}
		w.Header().Set("X-Ratelimit-Limit-Requests", "100")
		w.Header().Set("X-Ratelimit-Remaining-Requests", "75")
		w.Header().Set("X-Ratelimit-Reset-Requests", resetTime.Format(time.RFC3339))
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "test"})
	}))
	defer server.Close()

	probe := NewQuotaProbe(QuotaProbeConfig{HTTPClient: server.Client()})
	snapshot := probe.ProbeUsage(context.Background(), coreconfig.Config{
		Provider:  "openai-compatible",
		APIKey:    "test-key",
		APIBaseURL: server.URL,
		Model:     "gpt-4",
	})

	if !snapshot.Supported {
		t.Error("Supported = false, want true")
	}
	if !snapshot.Available {
		t.Error("Available = false, want true")
	}
	if snapshot.Provider != "openai-compatible" {
		t.Fatalf("Provider = %q, want openai-compatible", snapshot.Provider)
	}
	if snapshot.Status != "allowed" {
		t.Fatalf("Status = %q, want allowed", snapshot.Status)
	}
	if !snapshot.HasUtilization {
		t.Error("HasUtilization = false, want true")
	}
	wantUtil := 0.25 // 100-75 = 25 used, 25/100 = 0.25
	if snapshot.Utilization < wantUtil-0.01 || snapshot.Utilization > wantUtil+0.01 {
		t.Fatalf("Utilization = %f, want ~%f", snapshot.Utilization, wantUtil)
	}
	if snapshot.ResetsAt == 0 {
		t.Error("ResetsAt = 0, want non-zero")
	}
}

// TestQuotaProbeUsageWarning verifies allowed_warning status when remaining is low.
func TestQuotaProbeUsageWarning(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Ratelimit-Limit-Requests", "100")
		w.Header().Set("X-Ratelimit-Remaining-Requests", "5")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "test"})
	}))
	defer server.Close()

	probe := NewQuotaProbe(QuotaProbeConfig{HTTPClient: server.Client()})
	snapshot := probe.ProbeUsage(context.Background(), coreconfig.Config{
		Provider:   "openai-compatible",
		APIKey:     "test-key",
		APIBaseURL: server.URL,
		Model:      "gpt-4",
	})

	if snapshot.Status != "allowed_warning" {
		t.Fatalf("Status = %q, want allowed_warning", snapshot.Status)
	}
}

// TestQuotaProbeUsageRejected verifies rejected status when remaining is zero.
func TestQuotaProbeUsageRejected(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Ratelimit-Limit-Requests", "100")
		w.Header().Set("X-Ratelimit-Remaining-Requests", "0")
		w.WriteHeader(http.StatusTooManyRequests)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"message": "Rate limit exceeded", "type": "rate_limit_error"},
		})
	}))
	defer server.Close()

	probe := NewQuotaProbe(QuotaProbeConfig{HTTPClient: server.Client()})
	snapshot := probe.ProbeUsage(context.Background(), coreconfig.Config{
		Provider:   "openai-compatible",
		APIKey:     "test-key",
		APIBaseURL: server.URL,
		Model:      "gpt-4",
	})

	if snapshot.Status != "rejected" {
		t.Fatalf("Status = %q, want rejected", snapshot.Status)
	}
	if !snapshot.Available {
		t.Error("Available = false, want true (HTTP response received)")
	}
}

// TestQuotaProbeUsageUnreachable verifies graceful handling when endpoint is unreachable.
func TestQuotaProbeUsageUnreachable(t *testing.T) {
	probe := NewQuotaProbe(QuotaProbeConfig{HTTPClient: http.DefaultClient})
	snapshot := probe.ProbeUsage(context.Background(), coreconfig.Config{
		Provider:   "openai-compatible",
		APIKey:     "test-key",
		APIBaseURL: "http://localhost:1",
		Model:      "gpt-4",
	})

	if snapshot.Available {
		t.Error("Available = true, want false")
	}
	if !snapshot.Supported {
		t.Error("Supported = false, want true")
	}
	if snapshot.Summary == "" {
		t.Error("Summary is empty, want unreachable message")
	}
}

// TestQuotaProbeUsageNilProbe verifies nil probe returns unavailable.
func TestQuotaProbeUsageNilProbe(t *testing.T) {
	var probe *QuotaProbe
	snapshot := probe.ProbeUsage(context.Background(), coreconfig.Config{
		Provider: "openai-compatible",
		APIKey:   "test-key",
		Model:    "gpt-4",
	})

	if snapshot.Available {
		t.Error("Available = true, want false for nil probe")
	}
	if !snapshot.Supported {
		t.Error("Supported = false, want true")
	}
}

// TestQuotaProbeUsageGLMPath verifies GLM provider uses correct endpoint path.
func TestQuotaProbeUsageGLMPath(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "test"})
	}))
	defer server.Close()

	probe := NewQuotaProbe(QuotaProbeConfig{HTTPClient: server.Client()})
	_ = probe.ProbeUsage(context.Background(), coreconfig.Config{
		Provider:   "glm",
		APIKey:     "test-key",
		APIBaseURL: server.URL,
		Model:      "glm-4",
	})

	if gotPath != glmChatCompletionsPath {
		t.Fatalf("path = %q, want %q", gotPath, glmChatCompletionsPath)
	}
}

// TestParseOpenAILimitSnapshotNoHeaders verifies behavior when no rate-limit headers are present.
func TestParseOpenAILimitSnapshotNoHeaders(t *testing.T) {
	snapshot := parseOpenAILimitSnapshot(http.Header{})

	if snapshot.Status != "allowed" {
		t.Fatalf("Status = %q, want allowed", snapshot.Status)
	}
	if snapshot.HasUtilization {
		t.Error("HasUtilization = true, want false")
	}
	if snapshot.ResetsAt != 0 {
		t.Fatalf("ResetsAt = %d, want 0", snapshot.ResetsAt)
	}
}

// TestResolveQuotaProbeModel verifies model fallback when config model is empty.
func TestResolveQuotaProbeModel(t *testing.T) {
	if got := resolveQuotaProbeModel("gpt-4"); got != "gpt-4" {
		t.Fatalf("resolveQuotaProbeModel(gpt-4) = %q, want gpt-4", got)
	}
	if got := resolveQuotaProbeModel(""); got == "" {
		t.Fatal("resolveQuotaProbeModel(\"\") should fall back to default model")
	}
}
