package openai

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
	servicecommands "github.com/sheepzhao/claude-code-go/internal/services/commands"
)

const (
	// defaultStatusProbeTimeout bounds one /status connectivity request for OpenAI-compatible providers.
	defaultStatusProbeTimeout = 3 * time.Second
)

// StatusProbeConfig carries the dependencies needed by the OpenAI-compatible status probe.
type StatusProbeConfig struct {
	// Provider selects which OpenAI-compatible defaults the probe should use.
	Provider string
	// HTTPClient allows tests to inject a transport and production to reuse defaults.
	HTTPClient *http.Client
}

// StatusProbe performs the minimum OpenAI-compatible HTTP reachability check used by /status.
type StatusProbe struct {
	// provider stores the normalized provider used to resolve default hosts and paths.
	provider string
	// httpClient executes the short-lived probe request.
	httpClient *http.Client
}

// NewStatusProbe builds an OpenAI-compatible connectivity probe with a bounded timeout.
func NewStatusProbe(cfg StatusProbeConfig) *StatusProbe {
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultStatusProbeTimeout}
	}
	return &StatusProbe{
		provider:   coreconfig.NormalizeProvider(cfg.Provider),
		httpClient: httpClient,
	}
}

// Probe checks whether the configured OpenAI-compatible endpoint is reachable enough to return an HTTP response.
func (p *StatusProbe) Probe(ctx context.Context, cfg coreconfig.Config) servicecommands.APIConnectivityProbeResult {
	if p == nil || p.httpClient == nil {
		return servicecommands.APIConnectivityProbeResult{
			Summary: "probe unavailable",
		}
	}

	provider := coreconfig.NormalizeProvider(cfg.Provider)
	if provider == coreconfig.ProviderAnthropic {
		provider = p.provider
	}
	baseURL := strings.TrimRight(resolveBaseURL(provider, cfg.APIBaseURL), "/")
	path := resolveChatCompletionsPath(provider)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+path, nil)
	if err != nil {
		return servicecommands.APIConnectivityProbeResult{
			Summary: fmt.Sprintf("failed to build request (%v)", err),
		}
	}
	req.Header.Set("authorization", "Bearer "+cfg.APIKey)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return servicecommands.APIConnectivityProbeResult{
			Summary: fmt.Sprintf("unreachable (%v)", err),
		}
	}
	defer resp.Body.Close()

	return servicecommands.APIConnectivityProbeResult{
		Summary: fmt.Sprintf("reachable (HTTP %d from %s)", resp.StatusCode, path),
	}
}
