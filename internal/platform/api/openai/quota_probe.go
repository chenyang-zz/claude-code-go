package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
	servicecommands "github.com/sheepzhao/claude-code-go/internal/services/commands"
)

const (
	// defaultQuotaProbeTimeout bounds one minimal quota-observation request.
	defaultQuotaProbeTimeout = 5 * time.Second
)

// QuotaProbeConfig carries the dependencies needed by the OpenAI-compatible quota probe.
type QuotaProbeConfig struct {
	// HTTPClient allows tests to inject a transport and production to reuse defaults.
	HTTPClient *http.Client
}

// QuotaProbe performs the minimum OpenAI-compatible quota snapshot request used by usage-related slash commands.
type QuotaProbe struct {
	// httpClient executes the short-lived quota probe request.
	httpClient *http.Client
}

// NewQuotaProbe builds an OpenAI-compatible quota probe with a bounded timeout.
func NewQuotaProbe(cfg QuotaProbeConfig) *QuotaProbe {
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultQuotaProbeTimeout}
	}
	return &QuotaProbe{httpClient: httpClient}
}

// ProbeUsage checks whether the configured OpenAI-compatible endpoint can surface one stable rate-limit snapshot.
func (p *QuotaProbe) ProbeUsage(ctx context.Context, cfg coreconfig.Config) servicecommands.UsageLimitsSnapshot {
	snapshot := servicecommands.UsageLimitsSnapshot{
		Supported: true,
		Provider:  coreconfig.NormalizeProvider(cfg.Provider),
	}
	if p == nil || p.httpClient == nil {
		snapshot.Summary = "probe unavailable"
		return snapshot
	}

	// OpenAI-compatible providers do not expose a dedicated quota endpoint.
	// We send a minimal completions request and read the rate-limit headers from the response.
	body, err := json.Marshal(chatCompletionsRequest{
		Model:     resolveQuotaProbeModel(cfg.Model),
		Messages:  []chatMessage{{Role: "user", Content: "quota"}},
		Stream:    false,
		MaxTokens: 1,
	})
	if err != nil {
		snapshot.Summary = fmt.Sprintf("failed to marshal request (%v)", err)
		return snapshot
	}

	provider := coreconfig.NormalizeProvider(cfg.Provider)
	baseURL := strings.TrimRight(resolveBaseURL(provider, cfg.APIBaseURL), "/")
	path := resolveChatCompletionsPath(provider)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+path, bytes.NewReader(body))
	if err != nil {
		snapshot.Summary = fmt.Sprintf("failed to build request (%v)", err)
		return snapshot
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("accept", "application/json")
	if cfg.APIKey != "" {
		req.Header.Set("authorization", "Bearer "+cfg.APIKey)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		snapshot.Summary = fmt.Sprintf("unreachable (%v)", err)
		return snapshot
	}
	defer resp.Body.Close()

	snapshot = parseOpenAILimitSnapshot(resp.Header)
	snapshot.Supported = true
	snapshot.Provider = provider
	snapshot.Available = true
	snapshot.Summary = fmt.Sprintf("reachable (HTTP %d from %s)", resp.StatusCode, path)
	return snapshot
}

// resolveQuotaProbeModel selects one stable model identifier for the minimal quota probe.
func resolveQuotaProbeModel(model string) string {
	if strings.TrimSpace(model) != "" {
		return model
	}
	return coreconfig.DefaultConfig().Model
}

// parseOpenAILimitSnapshot converts OpenAI rate-limit headers into one stable command-facing snapshot.
func parseOpenAILimitSnapshot(headers http.Header) servicecommands.UsageLimitsSnapshot {
	snapshot := servicecommands.UsageLimitsSnapshot{}

	// OpenAI exposes rate-limit headers per resource (requests / tokens).
	// We surface the requests limit as the primary signal.
	remaining := parseHeaderInt(headers, "x-ratelimit-remaining-requests")
	limit := parseHeaderInt(headers, "x-ratelimit-limit-requests")

	if limit > 0 {
		snapshot.HasUtilization = true
		snapshot.Utilization = 1.0 - float64(remaining)/float64(limit)
	}

	// OpenAI returns reset timestamps as RFC3339 strings.
	if resetStr := strings.TrimSpace(headers.Get("x-ratelimit-reset-requests")); resetStr != "" {
		if t, err := time.Parse(time.RFC3339, resetStr); err == nil {
			snapshot.ResetsAt = t.Unix()
		}
	}

	// Infer status from remaining count only when limit header is present.
	if limit > 0 {
		if remaining == 0 {
			snapshot.Status = "rejected"
		} else if remaining <= limit/10 {
			snapshot.Status = "allowed_warning"
		} else {
			snapshot.Status = "allowed"
		}
	} else {
		snapshot.Status = "allowed"
	}

	return snapshot
}

// parseHeaderInt converts one optional integer header into an int value.
func parseHeaderInt(headers http.Header, key string) int {
	v := strings.TrimSpace(headers.Get(key))
	if v == "" {
		return 0
	}
	n, _ := strconv.Atoi(v)
	return n
}
