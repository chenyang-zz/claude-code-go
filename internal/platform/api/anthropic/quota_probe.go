package anthropic

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
	// quotaProbePath keeps the Anthropic quota probe pointed at the messages API used by the source implementation.
	quotaProbePath = "/v1/messages"
	// defaultQuotaProbeTimeout bounds one minimal quota-observation request.
	defaultQuotaProbeTimeout = 5 * time.Second
)

var earlyWarningClaimMap = map[string]string{
	"5h":      "five_hour",
	"7d":      "seven_day",
	"overage": "overage",
}

// QuotaProbeConfig carries the dependencies needed by the Anthropic quota probe.
type QuotaProbeConfig struct {
	// HTTPClient allows tests to inject a transport and production to reuse defaults.
	HTTPClient *http.Client
}

// QuotaProbe performs the minimum Anthropic quota snapshot request used by usage-related slash commands.
type QuotaProbe struct {
	// httpClient executes the short-lived quota probe request.
	httpClient *http.Client
}

// NewQuotaProbe builds an Anthropic quota probe with a bounded timeout.
func NewQuotaProbe(cfg QuotaProbeConfig) *QuotaProbe {
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultQuotaProbeTimeout}
	}
	return &QuotaProbe{httpClient: httpClient}
}

// ProbeUsage checks whether the configured Anthropic endpoint can surface one stable unified-limits snapshot.
func (p *QuotaProbe) ProbeUsage(ctx context.Context, cfg coreconfig.Config) servicecommands.UsageLimitsSnapshot {
	snapshot := servicecommands.UsageLimitsSnapshot{
		Supported: true,
		Provider:  coreconfig.NormalizeProvider(cfg.Provider),
	}
	if p == nil || p.httpClient == nil {
		snapshot.Summary = "probe unavailable"
		return snapshot
	}

	body, err := json.Marshal(messagesRequest{
		Model:     resolveQuotaProbeModel(cfg.Model),
		MaxTokens: 1,
		Stream:    false,
		Messages: []anthropicMessage{
			{
				Role: "user",
				Content: []anthropicContentBlock{
					{Type: "text", Text: "quota"},
				},
			},
		},
	})
	if err != nil {
		snapshot.Summary = fmt.Sprintf("failed to marshal request (%v)", err)
		return snapshot
	}

	baseURL := strings.TrimRight(cfg.APIBaseURL, "/")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+quotaProbePath, bytes.NewReader(body))
	if err != nil {
		snapshot.Summary = fmt.Sprintf("failed to build request (%v)", err)
		return snapshot
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("accept", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")
	if cfg.APIKey != "" {
		req.Header.Set("x-api-key", cfg.APIKey)
	}
	if cfg.AuthToken != "" {
		req.Header.Set("authorization", "Bearer "+cfg.AuthToken)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		snapshot.Summary = fmt.Sprintf("unreachable (%v)", err)
		return snapshot
	}
	defer resp.Body.Close()

	snapshot = parseUsageSnapshot(resp.Header)
	snapshot.Supported = true
	snapshot.Provider = coreconfig.ProviderAnthropic
	snapshot.Available = true
	snapshot.Summary = fmt.Sprintf("reachable (HTTP %d from %s)", resp.StatusCode, quotaProbePath)
	return snapshot
}

// resolveQuotaProbeModel selects one stable model identifier for the minimal quota probe.
func resolveQuotaProbeModel(model string) string {
	if strings.TrimSpace(model) != "" {
		return model
	}
	return coreconfig.DefaultConfig().Model
}

// parseUsageSnapshot converts Anthropic unified limit headers into one stable command-facing snapshot.
func parseUsageSnapshot(headers http.Header) servicecommands.UsageLimitsSnapshot {
	snapshot := servicecommands.UsageLimitsSnapshot{
		Status:                strings.TrimSpace(headers.Get("anthropic-ratelimit-unified-status")),
		RateLimitType:         strings.TrimSpace(headers.Get("anthropic-ratelimit-unified-representative-claim")),
		ResetsAt:              parseHeaderInt64(headers, "anthropic-ratelimit-unified-reset"),
		Utilization:           parseHeaderFloat(headers, "anthropic-ratelimit-unified-5h-utilization"),
		HasUtilization:        headers.Get("anthropic-ratelimit-unified-5h-utilization") != "",
		OverageStatus:         strings.TrimSpace(headers.Get("anthropic-ratelimit-unified-overage-status")),
		OverageResetsAt:       parseHeaderInt64(headers, "anthropic-ratelimit-unified-overage-reset"),
		OverageDisabledReason: strings.TrimSpace(headers.Get("anthropic-ratelimit-unified-overage-disabled-reason")),
		FallbackAvailable:     strings.EqualFold(strings.TrimSpace(headers.Get("anthropic-ratelimit-unified-fallback")), "available"),
	}

	if snapshot.Status == "" {
		snapshot.Status = "allowed"
	}
	if warningType, threshold := detectEarlyWarning(headers); warningType != "" {
		snapshot.Status = "allowed_warning"
		snapshot.RateLimitType = warningType
		if snapshot.ResetsAt == 0 {
			switch warningType {
			case "five_hour":
				snapshot.ResetsAt = parseHeaderInt64(headers, "anthropic-ratelimit-unified-5h-reset")
			case "seven_day":
				snapshot.ResetsAt = parseHeaderInt64(headers, "anthropic-ratelimit-unified-7d-reset")
			case "overage":
				snapshot.ResetsAt = parseHeaderInt64(headers, "anthropic-ratelimit-unified-overage-reset")
			}
		}
		if threshold > 0 {
			switch warningType {
			case "five_hour":
				snapshot.Utilization = parseHeaderFloat(headers, "anthropic-ratelimit-unified-5h-utilization")
				snapshot.HasUtilization = headers.Get("anthropic-ratelimit-unified-5h-utilization") != ""
			case "seven_day":
				snapshot.Utilization = parseHeaderFloat(headers, "anthropic-ratelimit-unified-7d-utilization")
				snapshot.HasUtilization = headers.Get("anthropic-ratelimit-unified-7d-utilization") != ""
			case "overage":
				snapshot.Utilization = parseHeaderFloat(headers, "anthropic-ratelimit-unified-overage-utilization")
				snapshot.HasUtilization = headers.Get("anthropic-ratelimit-unified-overage-utilization") != ""
			}
		}
	}

	return snapshot
}

// detectEarlyWarning checks whether the server reported one surpassed-threshold warning claim.
func detectEarlyWarning(headers http.Header) (string, float64) {
	for claim, mapped := range earlyWarningClaimMap {
		key := fmt.Sprintf("anthropic-ratelimit-unified-%s-surpassed-threshold", claim)
		value := strings.TrimSpace(headers.Get(key))
		if value == "" {
			continue
		}
		return mapped, parseStringFloat(value)
	}
	return "", 0
}

// parseHeaderInt64 converts one optional integer header into unix-seconds form.
func parseHeaderInt64(headers http.Header, key string) int64 {
	return parseStringInt64(strings.TrimSpace(headers.Get(key)))
}

// parseHeaderFloat converts one optional float header into the normalized 0-1 utilization value.
func parseHeaderFloat(headers http.Header, key string) float64 {
	return parseStringFloat(strings.TrimSpace(headers.Get(key)))
}

// parseStringInt64 converts one string into int64 while tolerating invalid values.
func parseStringInt64(value string) int64 {
	if value == "" {
		return 0
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0
	}
	return parsed
}

// parseStringFloat converts one string into float64 while tolerating invalid values.
func parseStringFloat(value string) float64 {
	if value == "" {
		return 0
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0
	}
	return parsed
}
