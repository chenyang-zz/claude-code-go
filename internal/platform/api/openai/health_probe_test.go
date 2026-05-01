package openai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/config"
	"github.com/sheepzhao/claude-code-go/internal/core/model"
)

func TestOpenAIHealthProbe_Healthy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("authorization"); got != "Bearer openai-key" {
			t.Fatalf("authorization = %q, want Bearer openai-key", got)
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	}))
	defer server.Close()

	probe := newOpenAIHealthProbe(config.Config{Provider: config.ProviderOpenAICompatible, APIBaseURL: server.URL, APIKey: "openai-key"}, server.Client())
	result := probe.Check(context.Background())

	if result.Provider != config.ProviderOpenAICompatible {
		t.Fatalf("provider = %q, want %q", result.Provider, config.ProviderOpenAICompatible)
	}
	if result.Status != model.HealthStatusHealthy {
		t.Fatalf("status = %q, want healthy", result.Status)
	}
}

func TestOpenAIHealthProbe_Unreachable(t *testing.T) {
	probe := newOpenAIHealthProbe(config.Config{Provider: config.ProviderOpenAICompatible, APIBaseURL: "http://localhost:1", APIKey: "k"}, nil)
	result := probe.Check(context.Background())

	if result.Status != model.HealthStatusUnhealthy {
		t.Fatalf("status = %q, want unhealthy", result.Status)
	}
}

func TestOpenAIHealthProbe_NilProbe(t *testing.T) {
	var probe *openAIHealthProbe
	result := probe.Check(context.Background())

	if result.Status != model.HealthStatusUnknown {
		t.Fatalf("status = %q, want unknown", result.Status)
	}
}

func TestOpenAIRegisterHealthProbes(t *testing.T) {
	hc := model.NewHealthChecker()
	RegisterHealthProbes(hc, config.Config{Provider: config.ProviderOpenAICompatible}, nil)
	if hc.Get(config.ProviderOpenAICompatible) == nil {
		t.Fatal("OpenAI probe not registered")
	}

	hc2 := model.NewHealthChecker()
	RegisterHealthProbes(hc2, config.Config{Provider: config.ProviderGLM}, nil)
	if hc2.Get(config.ProviderGLM) == nil {
		t.Fatal("GLM probe not registered")
	}
}

func TestOpenAIRegisterHealthProbes_SkipsAnthropic(t *testing.T) {
	hc := model.NewHealthChecker()
	RegisterHealthProbes(hc, config.Config{Provider: config.ProviderAnthropic}, nil)
	if hc.Get(config.ProviderOpenAICompatible) != nil {
		t.Fatal("OpenAI probe should not be registered for Anthropic provider")
	}
}
