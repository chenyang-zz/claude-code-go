package openai

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/config"
	"github.com/sheepzhao/claude-code-go/internal/core/model"
)

const defaultHealthProbeTimeout = 3 * time.Second

// openAIHealthProbe checks the OpenAI-compatible API health.
type openAIHealthProbe struct {
	provider   string
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// newOpenAIHealthProbe builds a health probe for an OpenAI-compatible provider.
func newOpenAIHealthProbe(cfg config.Config, httpClient *http.Client) *openAIHealthProbe {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultHealthProbeTimeout}
	}
	provider := config.NormalizeProvider(cfg.Provider)
	if provider == config.ProviderAnthropic {
		provider = config.ProviderOpenAICompatible
	}
	return &openAIHealthProbe{
		provider:   provider,
		baseURL:    strings.TrimRight(resolveBaseURL(provider, cfg.APIBaseURL), "/"),
		apiKey:     cfg.APIKey,
		httpClient: httpClient,
	}
}

// Check probes the OpenAI-compatible API and returns its health.
func (p *openAIHealthProbe) Check(ctx context.Context) model.HealthResult {
	if p == nil || p.httpClient == nil {
		provider := config.ProviderOpenAICompatible
		if p != nil {
			provider = p.provider
		}
		return model.HealthResult{
			Provider:  provider,
			Status:    model.HealthStatusUnknown,
			Message:   "probe unavailable",
			CheckedAt: time.Now(),
		}
	}

	path := resolveChatCompletionsPath(p.provider)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+path, nil)
	if err != nil {
		return model.HealthResult{
			Provider:  p.provider,
			Status:    model.HealthStatusUnhealthy,
			Message:   fmt.Sprintf("failed to build request (%v)", err),
			CheckedAt: time.Now(),
		}
	}
	if p.apiKey != "" {
		req.Header.Set("authorization", "Bearer "+p.apiKey)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return model.HealthResult{
			Provider:  p.provider,
			Status:    model.HealthStatusUnhealthy,
			Message:   fmt.Sprintf("unreachable (%v)", err),
			CheckedAt: time.Now(),
		}
	}
	defer resp.Body.Close()

	status := model.HealthStatusHealthy
	if resp.StatusCode >= 500 {
		status = model.HealthStatusDegraded
	}
	return model.HealthResult{
		Provider:  p.provider,
		Status:    status,
		Message:   fmt.Sprintf("reachable (HTTP %d from %s)", resp.StatusCode, path),
		CheckedAt: time.Now(),
	}
}

// RegisterHealthProbes registers the OpenAI-compatible provider health probe
// with the given HealthChecker when the provider is enabled in the configuration.
func RegisterHealthProbes(hc *model.HealthChecker, cfg config.Config, httpClient *http.Client) {
	provider := config.NormalizeProvider(cfg.Provider)
	if provider == config.ProviderOpenAICompatible || provider == config.ProviderGLM {
		hc.Register(provider, newOpenAIHealthProbe(cfg, httpClient))
	}
}
