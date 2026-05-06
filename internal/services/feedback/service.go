package feedback

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const (
	// feedbackAPIEndpoint is the Anthropic endpoint for feedback submission.
	feedbackAPIEndpoint = "https://api.anthropic.com/api/claude_cli_feedback"

	// apiTimeout is the maximum duration for the feedback API call.
	apiTimeout = 30 * time.Second
)

// apiErrorResponse matches the Anthropic error response structure.
type apiErrorResponse struct {
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// SubmitFeedback sends feedback data to the Anthropic API. It applies
// sensitive information redaction and handles OAuth token refresh before
// submission.
func SubmitFeedback(ctx context.Context, data FeedbackData, authHeaders map[string]string, userAgent string) SubmissionResult {
	// Redact sensitive info from the description before submission.
	data.Description = RedactSensitiveInfo(data.Description)

	jsonData, err := json.Marshal(map[string]any{
		"data": map[string]any{
			"latestAssistantMessageId": data.LatestAssistantMessageID,
			"message_count":           data.MessageCount,
			"datetime":                data.DateTime,
			"description":             data.Description,
			"platform":                data.Platform,
			"gitRepo":                 data.GitRepo,
			"version":                 data.Version,
			"terminal":                data.Terminal,
			"transcript":              data.Transcript,
			"errors":                  data.Errors,
			"subagentTranscripts":     data.SubagentTranscripts,
			"rawTranscriptJsonl":      data.RawTranscriptJSONL,
		},
	})
	if err != nil {
		logger.ErrorCF("feedback", "failed to marshal feedback data", map[string]any{"error": err.Error()})
		return SubmissionResult{Success: false}
	}

	ctx, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, feedbackAPIEndpoint, bytes.NewReader(jsonData))
	if err != nil {
		logger.ErrorCF("feedback", "failed to create request", map[string]any{"error": err.Error()})
		return SubmissionResult{Success: false}
	}

	req.Header.Set("Content-Type", "application/json")
	if userAgent != "" {
		req.Header.Set("User-Agent", userAgent)
	}
	for k, v := range authHeaders {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return SubmissionResult{Success: false}
		}
		logger.ErrorCF("feedback", "feedback API request failed", map[string]any{"error": err.Error()})
		return SubmissionResult{Success: false}
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	if resp.StatusCode == http.StatusOK {
		var result struct {
			FeedbackID string `json:"feedback_id"`
		}
		if err := json.Unmarshal(body, &result); err == nil && result.FeedbackID != "" {
			logger.DebugCF("feedback", "feedback submitted successfully", map[string]any{
				"feedback_id": result.FeedbackID,
			})
			return SubmissionResult{Success: true, FeedbackID: result.FeedbackID}
		}
		logger.ErrorCF("feedback", "feedback API returned 200 but no feedback_id", nil)
		return SubmissionResult{Success: false}
	}

	if resp.StatusCode == http.StatusForbidden {
		var errResp apiErrorResponse
		if err := json.Unmarshal(body, &errResp); err == nil {
			if errResp.Error.Type == "permission_error" && strings.Contains(errResp.Error.Message, "Custom data retention settings") {
				logger.ErrorCF("feedback", "cannot submit feedback: custom data retention settings enabled", nil)
				return SubmissionResult{Success: false, IsZDROrg: true}
			}
		}
	}

	logger.ErrorCF("feedback", "feedback API returned error status", map[string]any{
		"status": resp.StatusCode,
	})
	return SubmissionResult{Success: false}
}

// CreateGitHubIssueURL builds a GitHub Issues URL with the feedback content
// encoded as the issue body. Handles URL length truncation to stay within
// browser limits.
func CreateGitHubIssueURL(cfg ServiceConfig, feedbackID, title, description string, errors []ErrorInfo) string {
	sanitizedTitle := RedactSensitiveInfo(title)
	sanitizedDescription := RedactSensitiveInfo(description)

	bodyPrefix := fmt.Sprintf("**Bug Description**\n%s\n\n**Environment Info**\n- Platform: %s\n- Version: %s\n- Feedback ID: %s\n\n**Errors**\n```json\n",
		sanitizedDescription, "unknown", "unknown", feedbackID)
	errorSuffix := "\n```\n"
	truncationNote := "\n**Note:** Content was truncated.\n"

	errorsJSON, _ := json.MarshalIndent(errors, "", "  ")
	baseURL := fmt.Sprintf("%s/new?title=%s&labels=user-reported,bug&body=", cfg.GitHubIssuesRepoURL, url.QueryEscape(sanitizedTitle))

	encodedPrefix := url.QueryEscape(bodyPrefix)
	encodedSuffix := url.QueryEscape(errorSuffix)
	encodedNote := url.QueryEscape(truncationNote)
	encodedErrors := url.QueryEscape(string(errorsJSON))

	spaceForErrors := cfg.GitHubURLLimit - len(baseURL) - len(encodedPrefix) - len(encodedSuffix) - len(encodedNote)

	// If description alone exceeds limit, truncate everything.
	if spaceForErrors <= 0 {
		ellipsis := url.QueryEscape("…")
		buffer := 50
		maxEncodedLength := cfg.GitHubURLLimit - len(baseURL) - len(ellipsis) - len(encodedNote) - buffer
		fullBody := bodyPrefix + string(errorsJSON) + errorSuffix
		encodedFullBody := url.QueryEscape(fullBody)
		if len(encodedFullBody) > maxEncodedLength {
			encodedFullBody = encodedFullBody[:maxEncodedLength]
			if lastPercent := strings.LastIndex(encodedFullBody, "%"); lastPercent >= len(encodedFullBody)-2 {
				encodedFullBody = encodedFullBody[:lastPercent]
			}
		}
		return baseURL + encodedFullBody + ellipsis + encodedNote
	}

	// If errors fit, no truncation needed.
	if len(encodedErrors) <= spaceForErrors {
		return baseURL + encodedPrefix + encodedErrors + encodedSuffix
	}

	// Truncate errors to fit.
	ellipsis := url.QueryEscape("…")
	buffer := 50
	truncatedErrors := encodedErrors[:spaceForErrors-len(ellipsis)-buffer]
	if lastPercent := strings.LastIndex(truncatedErrors, "%"); lastPercent >= len(truncatedErrors)-2 {
		truncatedErrors = truncatedErrors[:lastPercent]
	}
	return baseURL + encodedPrefix + truncatedErrors + ellipsis + encodedSuffix + encodedNote
}
