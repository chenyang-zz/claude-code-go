package anthropic

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
)

// APIErrorType represents the Anthropic API error.type field values.
type APIErrorType string

const (
	// ErrorTypeInvalidRequest indicates a malformed or invalid request (400).
	ErrorTypeInvalidRequest APIErrorType = "invalid_request_error"
	// ErrorTypeAuthentication indicates an authentication failure (401).
	ErrorTypeAuthentication APIErrorType = "authentication_error"
	// ErrorTypePermission indicates a permission denial (403).
	ErrorTypePermission APIErrorType = "permission_error"
	// ErrorTypeNotFound indicates a resource not found (404).
	ErrorTypeNotFound APIErrorType = "not_found_error"
	// ErrorTypeRateLimit indicates a rate limit hit (429).
	ErrorTypeRateLimit APIErrorType = "rate_limit_error"
	// ErrorTypeOverloaded indicates the Anthropic API is overloaded (529).
	ErrorTypeOverloaded APIErrorType = "overloaded_error"
	// ErrorTypeAPIError indicates a generic internal server error (5xx).
	ErrorTypeAPIError APIErrorType = "api_error"
)

// APIError captures the structured error returned by the Anthropic API.
// It mirrors the Anthropic SDK's APIError shape for use in the Go runtime.
type APIError struct {
	// Status is the HTTP status code.
	Status int
	// Type is the Anthropic error.type field (e.g. "rate_limit_error").
	Type APIErrorType
	// Message is the human-readable error description.
	Message string
	// RequestID is the request-id header if present.
	RequestID string
	// Headers carries the raw HTTP response headers for retry-after / rate-limit parsing.
	Headers http.Header
	// RawBody is the original response body for diagnostics.
	RawBody []byte
}

// Error implements the error interface.
func (e *APIError) Error() string {
	if e.Type != "" {
		return fmt.Sprintf("anthropic api error: status=%d type=%s message=%s", e.Status, e.Type, e.Message)
	}
	return fmt.Sprintf("anthropic api error: status=%d message=%s", e.Status, e.Message)
}

// IsRetryable returns whether this error should trigger a retry.
// It follows the TS withRetry.ts shouldRetry logic aligned for the Go runtime.
func (e *APIError) IsRetryable() bool {
	if e == nil {
		return false
	}

	// Retry on overloaded errors (529 or overloaded_error type).
	if e.Status == 529 || e.Type == ErrorTypeOverloaded {
		return true
	}

	// Retry on rate limits.
	if e.Status == 429 || e.Type == ErrorTypeRateLimit {
		return true
	}

	// Retry on request timeouts.
	if e.Status == 408 {
		return true
	}

	// Retry on lock timeouts (409).
	if e.Status == 409 {
		return true
	}

	// Retry on 401 authentication errors (token refresh may fix it).
	if e.Status == 401 || e.Type == ErrorTypeAuthentication {
		return true
	}

	// Retry on 403 "OAuth token revoked" (same refresh logic as 401).
	if e.Status == 403 {
		if strings.Contains(e.Message, "OAuth token has been revoked") {
			return true
		}
	}

	// Retry all 5xx server errors.
	if e.Status >= 500 {
		return true
	}

	return false
}

// IsFatal returns whether this error should NOT be retried.
func (e *APIError) IsFatal() bool {
	if e == nil {
		return false
	}
	return !e.IsRetryable()
}

// RetryAfter returns the retry-after duration from response headers if present.
// It handles both seconds (integer) and RFC1123 date formats.
func (e *APIError) RetryAfter() time.Duration {
	if e == nil || e.Headers == nil {
		return 0
	}

	ra := strings.TrimSpace(e.Headers.Get("retry-after"))
	if ra == "" {
		return 0
	}

	// Try parsing as seconds first.
	if sec, err := strconv.Atoi(ra); err == nil && sec > 0 {
		return time.Duration(sec) * time.Second
	}

	// Try parsing as RFC1123 date.
	if t, err := time.Parse(time.RFC1123, ra); err == nil {
		d := time.Until(t)
		if d > 0 {
			return d
		}
		return 0
	}

	return 0
}

// RateLimitReset returns the unix timestamp from anthropic-ratelimit-unified-reset header.
func (e *APIError) RateLimitReset() int64 {
	if e == nil || e.Headers == nil {
		return 0
	}
	v := strings.TrimSpace(e.Headers.Get("anthropic-ratelimit-unified-reset"))
	if v == "" {
		return 0
	}
	n, _ := strconv.ParseInt(v, 10, 64)
	return n
}

// RateLimitRemaining returns the remaining requests from x-ratelimit-remaining header.
func (e *APIError) RateLimitRemaining() int {
	if e == nil || e.Headers == nil {
		return 0
	}
	v := strings.TrimSpace(e.Headers.Get("x-ratelimit-remaining"))
	if v == "" {
		return 0
	}
	n, _ := strconv.Atoi(v)
	return n
}

// IsOverloaded returns true if this is an overloaded error (529 status or overloaded_error type).
func (e *APIError) IsOverloaded() bool {
	if e == nil {
		return false
	}
	return e.Status == 529 || e.Type == ErrorTypeOverloaded
}

// IsRateLimit returns true if this is a rate limit error (429 status or rate_limit_error type).
func (e *APIError) IsRateLimit() bool {
	if e == nil {
		return false
	}
	return e.Status == 429 || e.Type == ErrorTypeRateLimit
}

// IsPromptTooLong returns true if this error indicates the prompt exceeds context window.
func (e *APIError) IsPromptTooLong() bool {
	if e == nil {
		return false
	}
	msg := strings.ToLower(e.Message)
	return strings.Contains(msg, "prompt is too long") ||
		strings.Contains(msg, "context_length_exceeded") ||
		strings.Contains(msg, "model_context_window_exceeded") ||
		strings.Contains(msg, "context_window_exceeded")
}

// IsAuthError returns true if this is an authentication/authorization error.
func (e *APIError) IsAuthError() bool {
	if e == nil {
		return false
	}
	return e.Status == 401 || e.Status == 403 ||
		e.Type == ErrorTypeAuthentication || e.Type == ErrorTypePermission
}

// anthropicErrorBody is the wire-format error response from the Anthropic API.
type anthropicErrorBody struct {
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// ParseAPIError parses an Anthropic API error from an HTTP response.
// It extracts the status code, error type, message, and relevant headers.
func ParseAPIError(resp *http.Response, body []byte) *APIError {
	if resp == nil {
		return nil
	}

	err := &APIError{
		Status:  resp.StatusCode,
		Headers: resp.Header.Clone(),
		RawBody: body,
	}

	// Extract request-id header if present.
	err.RequestID = resp.Header.Get("request-id")

	// Try to parse the JSON error body.
	var payload anthropicErrorBody
	if len(body) > 0 {
		if jsonErr := json.Unmarshal(body, &payload); jsonErr == nil {
			err.Type = APIErrorType(payload.Error.Type)
			err.Message = payload.Error.Message
		}
	}

	// Fallback: if JSON parsing failed or body is empty, use status text.
	if err.Message == "" {
		err.Message = http.StatusText(resp.StatusCode)
	}

	return err
}

// IsRetryableStatusCode returns true for HTTP status codes that are generally retryable.
func IsRetryableStatusCode(statusCode int) bool {
	switch statusCode {
	case 408, 429, 502, 503, 504, 529:
		return true
	default:
		return statusCode >= 500
	}
}

// IsFatalStatusCode returns true for HTTP status codes that should NOT be retried.
func IsFatalStatusCode(statusCode int) bool {
	switch statusCode {
	case 400, 401, 403, 404, 405, 422:
		return true
	default:
		return false
	}
}

// IsImageSizeError returns true when the API error message indicates a single
// image exceeds the maximum allowed size.
func IsImageSizeError(msg string) bool {
	return strings.Contains(msg, "image exceeds") && strings.Contains(msg, "maximum")
}

// IsManyImageDimensionError returns true when the API error message indicates
// an image exceeds the stricter 2000px dimension limit for many-image requests.
func IsManyImageDimensionError(msg string) bool {
	return strings.Contains(msg, "image dimensions exceed") && strings.Contains(msg, "many-image")
}

// IsMediaSizeError returns true for any media-size rejection including image
// size, many-image dimensions, or PDF page limits.
func IsMediaSizeError(msg string) bool {
	return IsImageSizeError(msg) || IsManyImageDimensionError(msg) ||
		strings.Contains(msg, "maximum of") && strings.Contains(msg, "PDF pages")
}

// MaxMediaPerRequest is the maximum number of media items (images + documents)
// allowed in a single Anthropic API request.
const MaxMediaPerRequest = 100

// CountMediaItems counts the total number of image and document content blocks
// across all messages in a request.
func CountMediaItems(messages []message.Message) int {
	count := 0
	for _, msg := range messages {
		for _, part := range msg.Content {
			if part.Type == "image" || part.Type == "document" {
				count++
			}
		}
	}
	return count
}
