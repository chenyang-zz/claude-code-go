package anthropic

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
	// statusProbePath keeps the connectivity probe pointed at one stable Anthropic API route.
	statusProbePath = "/v1/messages"
	// defaultStatusProbeTimeout bounds one /status connectivity request.
	defaultStatusProbeTimeout = 3 * time.Second
)

// StatusProbeConfig carries the dependencies needed by the Anthropic status probe.
type StatusProbeConfig struct {
	// HTTPClient allows tests to inject a transport and production to reuse defaults.
	HTTPClient *http.Client
}

// StatusProbe performs the minimum Anthropic HTTP reachability check used by /status.
type StatusProbe struct {
	// httpClient executes the short-lived probe request.
	httpClient *http.Client
}

// NewStatusProbe builds an Anthropic connectivity probe with a bounded timeout.
func NewStatusProbe(cfg StatusProbeConfig) *StatusProbe {
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultStatusProbeTimeout}
	}
	return &StatusProbe{httpClient: httpClient}
}

// Probe checks whether the configured Anthropic endpoint is reachable enough to return an HTTP response.
func (p *StatusProbe) Probe(ctx context.Context, cfg coreconfig.Config) servicecommands.APIConnectivityProbeResult {
	if p == nil || p.httpClient == nil {
		return servicecommands.APIConnectivityProbeResult{
			Summary: "probe unavailable",
		}
	}

	baseURL := strings.TrimRight(cfg.APIBaseURL, "/")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+statusProbePath, nil)
	if err != nil {
		return servicecommands.APIConnectivityProbeResult{
			Summary: fmt.Sprintf("failed to build request (%v)", err),
		}
	}
	req.Header.Set("x-api-key", cfg.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return servicecommands.APIConnectivityProbeResult{
			Summary: fmt.Sprintf("unreachable (%v)", err),
		}
	}
	defer resp.Body.Close()

	return servicecommands.APIConnectivityProbeResult{
		Summary: fmt.Sprintf("reachable (HTTP %d from %s)", resp.StatusCode, statusProbePath),
	}
}
