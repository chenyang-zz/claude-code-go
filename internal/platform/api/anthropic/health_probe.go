package anthropic

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

// anthropicHealthProbe checks the Anthropic first-party API health.
type anthropicHealthProbe struct {
	baseURL    string
	apiKey     string
	authToken  string
	httpClient *http.Client
}

// newAnthropicHealthProbe builds a health probe for the Anthropic first-party API.
func newAnthropicHealthProbe(cfg config.Config, httpClient *http.Client) *anthropicHealthProbe {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultHealthProbeTimeout}
	}
	baseURL := strings.TrimRight(cfg.APIBaseURL, "/")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &anthropicHealthProbe{
		baseURL:    baseURL,
		apiKey:     cfg.APIKey,
		authToken:  cfg.AuthToken,
		httpClient: httpClient,
	}
}

// Check probes the Anthropic API and returns its health.
func (p *anthropicHealthProbe) Check(ctx context.Context) model.HealthResult {
	if p == nil || p.httpClient == nil {
		return model.HealthResult{
			Provider:  config.ProviderAnthropic,
			Status:    model.HealthStatusUnknown,
			Message:   "probe unavailable",
			CheckedAt: time.Now(),
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/v1/messages", nil)
	if err != nil {
		return model.HealthResult{
			Provider:  config.ProviderAnthropic,
			Status:    model.HealthStatusUnhealthy,
			Message:   fmt.Sprintf("failed to build request (%v)", err),
			CheckedAt: time.Now(),
		}
	}
	if p.apiKey != "" {
		req.Header.Set("x-api-key", p.apiKey)
	}
	if p.authToken != "" {
		req.Header.Set("authorization", "Bearer "+p.authToken)
	}
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return model.HealthResult{
			Provider:  config.ProviderAnthropic,
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
		Provider:  config.ProviderAnthropic,
		Status:    status,
		Message:   fmt.Sprintf("reachable (HTTP %d from /v1/messages)", resp.StatusCode),
		CheckedAt: time.Now(),
	}
}

// vertexHealthProbe checks the Google Cloud Vertex AI API health.
type vertexHealthProbe struct {
	region     string
	projectID  string
	auth       GoogleAuthenticator
	httpClient *http.Client
}

// newVertexHealthProbe builds a health probe for Vertex AI.
func newVertexHealthProbe(cfg config.Config, httpClient *http.Client) *vertexHealthProbe {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultHealthProbeTimeout}
	}
	region := resolveVertexRegion("")
	if v := cfg.VertexRegion; v != "" {
		region = v
	}
	return &vertexHealthProbe{
		region:     region,
		projectID:  cfg.VertexProjectID,
		auth:       newGoogleAuthenticator(cfg.VertexSkipAuth),
		httpClient: httpClient,
	}
}

// Check probes the Vertex AI API and returns its health.
func (p *vertexHealthProbe) Check(ctx context.Context) model.HealthResult {
	if p == nil || p.httpClient == nil {
		return model.HealthResult{
			Provider:  config.ProviderVertex,
			Status:    model.HealthStatusUnknown,
			Message:   "probe unavailable",
			CheckedAt: time.Now(),
		}
	}

	if p.projectID == "" {
		return model.HealthResult{
			Provider:  config.ProviderVertex,
			Status:    model.HealthStatusNotConfigured,
			Message:   "missing project ID",
			CheckedAt: time.Now(),
		}
	}

	// Probe the regional aiplatform endpoint.
	endpoint := fmt.Sprintf("https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s", p.region, p.projectID, p.region)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return model.HealthResult{
			Provider:  config.ProviderVertex,
			Status:    model.HealthStatusUnhealthy,
			Message:   fmt.Sprintf("failed to build request (%v)", err),
			CheckedAt: time.Now(),
		}
	}

	if p.auth != nil {
		token, err := p.auth.GetToken(ctx)
		if err != nil {
			return model.HealthResult{
				Provider:  config.ProviderVertex,
				Status:    model.HealthStatusUnhealthy,
				Message:   fmt.Sprintf("auth failed (%v)", err),
				CheckedAt: time.Now(),
			}
		}
		if token != "" {
			req.Header.Set("authorization", "Bearer "+token)
		}
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return model.HealthResult{
			Provider:  config.ProviderVertex,
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
		Provider:  config.ProviderVertex,
		Status:    status,
		Message:   fmt.Sprintf("reachable (HTTP %d from %s)", resp.StatusCode, endpoint),
		CheckedAt: time.Now(),
	}
}

// bedrockHealthProbe checks the AWS Bedrock API health.
type bedrockHealthProbe struct {
	region     string
	httpClient *http.Client
}

// newBedrockHealthProbe builds a health probe for AWS Bedrock.
func newBedrockHealthProbe(_ config.Config, httpClient *http.Client) *bedrockHealthProbe {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultHealthProbeTimeout}
	}
	return &bedrockHealthProbe{
		region:     resolveBedrockRegion(),
		httpClient: httpClient,
	}
}

// Check probes the Bedrock API and returns its health.
func (p *bedrockHealthProbe) Check(ctx context.Context) model.HealthResult {
	if p == nil || p.httpClient == nil {
		return model.HealthResult{
			Provider:  config.ProviderBedrock,
			Status:    model.HealthStatusUnknown,
			Message:   "probe unavailable",
			CheckedAt: time.Now(),
		}
	}

	// Bedrock does not have a simple public health endpoint that can be hit
	// without a valid AWS Signature V4 request. We probe the runtime endpoint
	// with an unsigned HEAD request; a 403 from AWS confirms the endpoint
	// is reachable even if the request is rejected for lacking a signature.
	endpoint := fmt.Sprintf("https://bedrock-runtime.%s.amazonaws.com", p.region)
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, endpoint, nil)
	if err != nil {
		return model.HealthResult{
			Provider:  config.ProviderBedrock,
			Status:    model.HealthStatusUnhealthy,
			Message:   fmt.Sprintf("failed to build request (%v)", err),
			CheckedAt: time.Now(),
		}
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return model.HealthResult{
			Provider:  config.ProviderBedrock,
			Status:    model.HealthStatusUnhealthy,
			Message:   fmt.Sprintf("unreachable (%v)", err),
			CheckedAt: time.Now(),
		}
	}
	defer resp.Body.Close()

	// A 403 from Bedrock means the endpoint is reachable but auth is required,
	// which is expected for an unsigned probe. Any other 4xx or 2xx also
	// indicates reachability.
	status := model.HealthStatusHealthy
	if resp.StatusCode >= 500 {
		status = model.HealthStatusDegraded
	}
	return model.HealthResult{
		Provider:  config.ProviderBedrock,
		Status:    status,
		Message:   fmt.Sprintf("reachable (HTTP %d from %s)", resp.StatusCode, endpoint),
		CheckedAt: time.Now(),
	}
}

// foundryHealthProbe checks the Azure AI Foundry API health.
type foundryHealthProbe struct {
	baseURL    string
	apiKey     string
	skipAuth   bool
	httpClient *http.Client
}

// newFoundryHealthProbe builds a health probe for Azure AI Foundry.
func newFoundryHealthProbe(cfg config.Config, httpClient *http.Client) *foundryHealthProbe {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultHealthProbeTimeout}
	}
	baseURL, _ := resolveFoundryBaseURL(cfg.FoundryResource, cfg.FoundryBaseURL)
	return &foundryHealthProbe{
		baseURL:    baseURL,
		apiKey:     cfg.FoundryAPIKey,
		skipAuth:   cfg.FoundrySkipAuth,
		httpClient: httpClient,
	}
}

// Check probes the Foundry API and returns its health.
func (p *foundryHealthProbe) Check(ctx context.Context) model.HealthResult {
	if p == nil || p.httpClient == nil {
		return model.HealthResult{
			Provider:  config.ProviderFoundry,
			Status:    model.HealthStatusUnknown,
			Message:   "probe unavailable",
			CheckedAt: time.Now(),
		}
	}

	if p.baseURL == "" {
		return model.HealthResult{
			Provider:  config.ProviderFoundry,
			Status:    model.HealthStatusNotConfigured,
			Message:   "missing endpoint configuration",
			CheckedAt: time.Now(),
		}
	}

	endpoint := buildFoundryEndpoint(p.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return model.HealthResult{
			Provider:  config.ProviderFoundry,
			Status:    model.HealthStatusUnhealthy,
			Message:   fmt.Sprintf("failed to build request (%v)", err),
			CheckedAt: time.Now(),
		}
	}
	if p.apiKey != "" {
		req.Header.Set("api-key", p.apiKey)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return model.HealthResult{
			Provider:  config.ProviderFoundry,
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
		Provider:  config.ProviderFoundry,
		Status:    status,
		Message:   fmt.Sprintf("reachable (HTTP %d from %s)", resp.StatusCode, endpoint),
		CheckedAt: time.Now(),
	}
}

// RegisterHealthProbes registers all anthropic-package provider health probes
// (Anthropic, Vertex, Bedrock, Foundry) with the given HealthChecker when the
// corresponding provider is enabled in the configuration.
func RegisterHealthProbes(hc *model.HealthChecker, cfg config.Config, httpClient *http.Client) {
	provider := config.NormalizeProvider(cfg.Provider)
	if provider == config.ProviderAnthropic || provider == "" {
		hc.Register(config.ProviderAnthropic, newAnthropicHealthProbe(cfg, httpClient))
	}
	if provider == config.ProviderVertex || cfg.VertexProjectID != "" {
		hc.Register(config.ProviderVertex, newVertexHealthProbe(cfg, httpClient))
	}
	if provider == config.ProviderBedrock || cfg.BedrockRegion != "" {
		hc.Register(config.ProviderBedrock, newBedrockHealthProbe(cfg, httpClient))
	}
	if provider == config.ProviderFoundry || cfg.FoundryResource != "" || cfg.FoundryBaseURL != "" {
		hc.Register(config.ProviderFoundry, newFoundryHealthProbe(cfg, httpClient))
	}
}
