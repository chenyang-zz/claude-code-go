package openai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
)

// TestStatusProbeProbeReportsReachable verifies the OpenAI-compatible status probe reports HTTP reachability.
func TestStatusProbeProbeReportsReachable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != defaultChatCompletionsPath {
			t.Fatalf("request path = %q, want %q", r.URL.Path, defaultChatCompletionsPath)
		}
		if r.Method != http.MethodGet {
			t.Fatalf("request method = %q, want GET", r.Method)
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	}))
	defer server.Close()

	probe := NewStatusProbe(StatusProbeConfig{
		Provider:   coreconfig.ProviderOpenAICompatible,
		HTTPClient: server.Client(),
	})
	result := probe.Probe(context.Background(), coreconfig.Config{
		Provider:   coreconfig.ProviderOpenAICompatible,
		APIKey:     "test-key",
		APIBaseURL: server.URL,
	})

	if result.Summary != "reachable (HTTP 405 from /v1/chat/completions)" {
		t.Fatalf("Probe() summary = %q, want reachable HTTP summary", result.Summary)
	}
}

// TestStatusProbeProbeResolvesGLMDefaults verifies the GLM probe uses the GLM-specific endpoint path.
func TestStatusProbeProbeResolvesGLMDefaults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != glmChatCompletionsPath {
			t.Fatalf("request path = %q, want %q", r.URL.Path, glmChatCompletionsPath)
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	}))
	defer server.Close()

	probe := NewStatusProbe(StatusProbeConfig{
		Provider:   coreconfig.ProviderGLM,
		HTTPClient: server.Client(),
	})
	result := probe.Probe(context.Background(), coreconfig.Config{
		Provider:   coreconfig.ProviderGLM,
		APIKey:     "test-key",
		APIBaseURL: server.URL,
	})

	if result.Summary != "reachable (HTTP 405 from /v4/chat/completions)" {
		t.Fatalf("Probe() summary = %q, want reachable HTTP summary", result.Summary)
	}
}
