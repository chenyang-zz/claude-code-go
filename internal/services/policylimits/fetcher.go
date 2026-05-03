package policylimits

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/platform/oauth"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const (
	fetchTimeout      = 10 * time.Second
	defaultMaxRetries = 5
	baseDelay         = 500 * time.Millisecond
	maxDelay          = 30 * time.Second
	jitterFrac        = 0.25
)

// Fetch retrieves policy limits from the Anthropic API with retry and ETag
// support. Returns nil restrictions with a nil error when the server responds
// 304 Not Modified, signalling the caller to use its cached version.
func Fetch(ctx context.Context, apiKey, accessToken, baseURL, cachedChecksum string) (*PolicyLimitsResponse, error) {
	if baseURL == "" {
		baseURL = oauth.DefaultBaseAPIURL
	}

	var lastErr error
	for attempt := 0; attempt <= defaultMaxRetries; attempt++ {
		resp, err := fetchOnce(ctx, apiKey, accessToken, baseURL, cachedChecksum)
		if err == nil {
			return resp, nil
		}
		lastErr = err

		if attempt >= defaultMaxRetries {
			break
		}

		delay := exponentialBackoff(attempt)
		logger.DebugCF("policylimits", "fetch retry", map[string]any{
			"attempt":  attempt + 1,
			"delay_ms": delay.Milliseconds(),
		})
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}
	return nil, lastErr
}

func fetchOnce(ctx context.Context, apiKey, accessToken, baseURL, cachedChecksum string) (*PolicyLimitsResponse, error) {
	url := baseURL + "/api/claude_code/policy_limits"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	if apiKey != "" {
		req.Header.Set("x-api-key", apiKey)
	} else if accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken)
		req.Header.Set("anthropic-beta", oauth.OAuthBetaHeader)
	} else {
		return nil, fmt.Errorf("no authentication available")
	}

	if cachedChecksum != "" {
		req.Header.Set("If-None-Match", `"`+cachedChecksum+`"`)
	}

	client := &http.Client{Timeout: fetchTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNotModified:
		return nil, nil // signal caller to use cached version
	case http.StatusNotFound:
		return &PolicyLimitsResponse{Restrictions: map[string]Restriction{}}, nil
	case http.StatusOK:
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		var result PolicyLimitsResponse
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, fmt.Errorf("invalid policy limits format: %w", err)
		}
		return &result, nil
	default:
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("policy limits request failed: status=%d body=%s", resp.StatusCode, string(body))
	}
}

func exponentialBackoff(attempt int) time.Duration {
	exp := time.Duration(float64(baseDelay) * math.Pow(2, float64(attempt)))
	if exp > maxDelay {
		exp = maxDelay
	}
	jitter := time.Duration(rand.Int63n(int64(float64(exp) * jitterFrac)))
	return exp + jitter
}
