package settingssync

import (
	"bytes"
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
	// syncTimeout is the HTTP client timeout for settings sync requests.
	syncTimeout = 10 * time.Second
	// defaultMaxRetries controls the maximum number of retries for fetch.
	defaultMaxRetries = 3
	// baseDelay is the starting backoff duration before jitter is applied.
	baseDelay = 500 * time.Millisecond
	// maxDelay caps the exponential backoff.
	maxDelay = 30 * time.Second
	// jitterFraction controls the ±jitter range as a fraction of the computed delay.
	jitterFraction = 0.25
)

// fetchUserSettingsGET performs a single GET request to retrieve user settings
// from the backend. It handles OAuth token refresh, auth headers, and response
// classification (200/404/401/5xx).
func fetchUserSettingsGET(ctx context.Context, baseURL, accessToken string) (*SettingsSyncFetchResult, error) {
	url := baseURL + "/api/claude_code/user_settings"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("anthropic-beta", oauth.OAuthBetaHeader)

	client := &http.Client{Timeout: syncTimeout}
	resp, err := client.Do(req)
	if err != nil {
		kind := classifyHTTPErr(err)
		if kind == "auth" {
			return &SettingsSyncFetchResult{
				Success:   false,
				Error:     "Not authorized for settings sync",
				SkipRetry: true,
			}, nil
		}
		return &SettingsSyncFetchResult{
			Success: false,
			Error:   "Cannot connect to server",
		}, nil
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1 MB limit

	// 404 means no settings exist yet — not an error.
	if resp.StatusCode == http.StatusNotFound {
		logger.DebugCF("settingssync", "fetch returned 404 - no settings exist yet", nil)
		return &SettingsSyncFetchResult{
			Success: true,
			IsEmpty: true,
		}, nil
	}

	// 401/403 — auth failure, don't retry.
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return &SettingsSyncFetchResult{
			Success:   false,
			Error:     fmt.Sprintf("Not authorized for settings sync (HTTP %d)", resp.StatusCode),
			SkipRetry: true,
		}, nil
	}

	// Non-200 response — retryable.
	if resp.StatusCode != http.StatusOK {
		return &SettingsSyncFetchResult{
			Success: false,
			Error:   fmt.Sprintf("Unexpected HTTP status %d", resp.StatusCode),
		}, nil
	}

	var data UserSyncData
	if err := json.Unmarshal(body, &data); err != nil {
		logger.WarnCF("settingssync", "invalid settings sync response format", map[string]any{
			"error": err.Error(),
		})
		return &SettingsSyncFetchResult{
			Success: false,
			Error:   "Invalid settings sync response format",
		}, nil
	}

	logger.DebugCF("settingssync", "fetch succeeded", map[string]any{
		"user_id":   data.UserID,
		"version":   data.Version,
		"entry_cnt": len(data.Content.Entries),
	})
	return &SettingsSyncFetchResult{
		Success: true,
		Data:    &data,
	}, nil
}

// FetchUserSettings performs GET with retry (up to maxRetries attempts).
// Returns the fetched data, an isEmpty indicator, or an error.
func FetchUserSettings(ctx context.Context, baseURL, accessToken string, maxRetries int) (*SettingsSyncFetchResult, error) {
	var lastResult *SettingsSyncFetchResult

	for attempt := 0; attempt <= maxRetries; attempt++ {
		result, err := fetchUserSettingsGET(ctx, baseURL, accessToken)
		if err != nil {
			return nil, err
		}
		lastResult = result

		if result.Success {
			return result, nil
		}

		if result.SkipRetry {
			return result, nil
		}

		// No more retries left.
		if attempt >= maxRetries {
			return result, nil
		}

		delay := exponentialBackoff(attempt)
		logger.DebugCF("settingssync", "fetch retry", map[string]any{
			"attempt":  attempt + 1,
			"max":      maxRetries,
			"delay_ms": delay.Milliseconds(),
		})
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}

	return lastResult, nil
}

// UploadUserSettings performs a PUT request to upload the given entries map
// to the backend. Returns the upload result with checksum and lastModified.
func UploadUserSettings(ctx context.Context, baseURL, accessToken string, entries map[string]string) (*SettingsSyncUploadResult, error) {
	url := baseURL + "/api/claude_code/user_settings"

	bodyBytes, err := json.Marshal(map[string]interface{}{
		"entries": entries,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal upload body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("anthropic-beta", oauth.OAuthBetaHeader)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: syncTimeout}
	resp, err := client.Do(req)
	if err != nil {
		logger.WarnCF("settingssync", "upload request failed", map[string]any{
			"error": err.Error(),
		})
		return &SettingsSyncUploadResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
		errMsg := fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body))
		logger.WarnCF("settingssync", "upload failed", map[string]any{
			"status": resp.StatusCode,
		})
		return &SettingsSyncUploadResult{
			Success: false,
			Error:   errMsg,
		}, nil
	}

	var uploadResp struct {
		Checksum     string `json:"checksum"`
		LastModified string `json:"lastModified"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&uploadResp); err != nil {
		// Non-fatal: the upload succeeded even if we can't parse the response.
		logger.DebugCF("settingssync", "upload succeeded but response parse failed", map[string]any{
			"error": err.Error(),
		})
		return &SettingsSyncUploadResult{Success: true}, nil
	}

	logger.DebugCF("settingssync", "upload succeeded", map[string]any{
		"entry_cnt":    len(entries),
		"lastModified": uploadResp.LastModified,
	})
	return &SettingsSyncUploadResult{
		Success:      true,
		Checksum:     uploadResp.Checksum,
		LastModified: uploadResp.LastModified,
	}, nil
}

// exponentialBackoff computes a retry delay using baseDelay * 2^attempt with
// ±jitter and an upper cap of maxDelay.
func exponentialBackoff(attempt int) time.Duration {
	exp := float64(uint(1) << attempt)
	delay := baseDelay.Seconds() * exp
	if delay > maxDelay.Seconds() {
		delay = maxDelay.Seconds()
	}
	jitter := delay * jitterFraction * (2*rand.Float64() - 1)
	delay += jitter
	if delay < 0 {
		delay = baseDelay.Seconds()
	}
	return time.Duration(math.Ceil(delay * float64(time.Second)))
}

// classifyHTTPErr maps common HTTP client errors to a category.
func classifyHTTPErr(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	if containsAny(msg, "timeout", "deadline exceeded") {
		return "timeout"
	}
	return "network"
}

func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if len(s) >= len(sub) {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}

// OAuthBaseURL derives the OAuth base API URL from configuration.
// Falls back to the default Anthropic OAuth API when not configured.
func OAuthBaseURL() string {
	c := getConfig()
	if c != nil && c.APIBaseURL != "" {
		return c.APIBaseURL
	}
	return oauth.DefaultBaseAPIURL
}
