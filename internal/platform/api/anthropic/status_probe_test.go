package anthropic

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
)

// TestStatusProbeProbeReportsReachable verifies the Anthropic status probe reports HTTP reachability.
func TestStatusProbeProbeReportsReachable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != statusProbePath {
			t.Fatalf("request path = %q, want %q", r.URL.Path, statusProbePath)
		}
		if r.Method != http.MethodGet {
			t.Fatalf("request method = %q, want GET", r.Method)
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	}))
	defer server.Close()

	probe := NewStatusProbe(StatusProbeConfig{
		HTTPClient: server.Client(),
	})
	result := probe.Probe(context.Background(), coreconfig.Config{
		APIKey:     "test-key",
		APIBaseURL: server.URL,
	})

	if result.Summary != "reachable (HTTP 405 from /v1/messages)" {
		t.Fatalf("Probe() summary = %q, want reachable HTTP summary", result.Summary)
	}
}

// TestStatusProbeProbeReportsNetworkFailure verifies the probe returns one stable unreachable summary on transport errors.
func TestStatusProbeProbeReportsNetworkFailure(t *testing.T) {
	probe := NewStatusProbe(StatusProbeConfig{
		HTTPClient: &http.Client{},
	})
	result := probe.Probe(context.Background(), coreconfig.Config{
		APIKey:     "test-key",
		APIBaseURL: "http://127.0.0.1:1",
	})

	if result.Summary == "" || result.Summary[:11] != "unreachable" {
		t.Fatalf("Probe() summary = %q, want unreachable prefix", result.Summary)
	}
}
