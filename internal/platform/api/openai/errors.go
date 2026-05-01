package openai

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// APIErrorType represents the OpenAI API error.type field values.
type APIErrorType string

const (
	// ErrorTypeInvalidRequest indicates a malformed or invalid request (400).
	ErrorTypeInvalidRequest APIErrorType = "invalid_request_error"
	// ErrorTypeAuthentication indicates an authentication failure (401).
	ErrorTypeAuthentication APIErrorType = "authentication_error"
	// ErrorTypePermission indicates a permission denial or insufficient quota (403/429).
	ErrorTypePermission APIErrorType = "insufficient_quota"
	// ErrorTypeRateLimit indicates a rate limit hit (429).
	ErrorTypeRateLimit APIErrorType = "rate_limit_error"
	// ErrorTypeServerError indicates an internal server error (5xx).
	ErrorTypeServerError APIErrorType = "server_error"
)

// APIError captures the structured error returned by the OpenAI API.
// It mirrors the OpenAI error envelope for use in the Go runtime.
type APIError struct {
	// Status is the HTTP status code.
	Status int
	// Type is the OpenAI error.type field (e.g. "rate_limit_error").
	Type APIErrorType
	// Message is the human-readable error description.
	Message string
	// Param is the parameter that caused the error, if any.
	Param string
	// Code is the provider-specific error code, if any.
	Code string
	// Headers carries the raw HTTP response headers for retry-after / rate-limit parsing.
	Headers http.Header
	// RawBody is the original response body for diagnostics.
	RawBody []byte
}

// Error implements the error interface.
func (e *APIError) Error() string {
	if e.Type != "" {
		return fmt.Sprintf("openai api error: status=%d type=%s message=%s", e.Status, e.Type, e.Message)
	}
	return fmt.Sprintf("openai api error: status=%d message=%s", e.Status, e.Message)
}

// IsRetryable returns whether this error should trigger a retry.
// It follows the OpenAI API semantics aligned for the Go runtime.
func (e *APIError) IsRetryable() bool {
	if e == nil {
		return false
	}

	// Retry on rate limits (429).
	if e.Status == 429 || e.Type == ErrorTypeRateLimit {
		return true
	}

	// Retry on request timeouts (408).
	if e.Status == 408 {
		return true
	}

	// Retry on authentication errors (401) — token may be refreshable.
	if e.Status == 401 || e.Type == ErrorTypeAuthentication {
		return true
	}

	// Retry all 5xx server errors.
	if e.Status >= 500 {
		return true
	}

	return false
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

// RateLimitRemaining returns the remaining requests from x-ratelimit-remaining-requests header.
func (e *APIError) RateLimitRemaining() int {
	if e == nil || e.Headers == nil {
		return 0
	}
	v := strings.TrimSpace(e.Headers.Get("x-ratelimit-remaining-requests"))
	if v == "" {
		return 0
	}
	n, _ := strconv.Atoi(v)
	return n
}

// RateLimitReset returns the reset timestamp from x-ratelimit-reset-requests header.
// The value is an RFC3339 timestamp string.
func (e *APIError) RateLimitReset() time.Time {
	if e == nil || e.Headers == nil {
		return time.Time{}
	}
	v := strings.TrimSpace(e.Headers.Get("x-ratelimit-reset-requests"))
	if v == "" {
		return time.Time{}
	}
	t, _ := time.Parse(time.RFC3339, v)
	return t
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
	return strings.Contains(msg, "context length exceeded") ||
		strings.Contains(msg, "context_length_exceeded") ||
		strings.Contains(msg, "maximum context length")
}

// IsAuthError returns true if this is an authentication/authorization error.
func (e *APIError) IsAuthError() bool {
	if e == nil {
		return false
	}
	return e.Status == 401 || e.Status == 403 ||
		e.Type == ErrorTypeAuthentication || e.Type == ErrorTypePermission
}

// IsImageSizeError reports whether this error indicates an OpenAI Vision input
// rejected for being too large (file size exceeds the per-image upload limit).
// Mirrors the semantics of the Anthropic-side IsImageSizeError introduced in
// batch-224 so cross-Provider retry / fallback decisions stay consistent.
func (e *APIError) IsImageSizeError() bool {
	if e == nil {
		return false
	}
	code := strings.ToLower(e.Code)
	if code == "image_too_large" || code == "media_size_exceeded" {
		return true
	}
	msg := strings.ToLower(e.Message)
	return strings.Contains(msg, "image too large") ||
		strings.Contains(msg, "image is too large") ||
		strings.Contains(msg, "media size exceeded") ||
		strings.Contains(msg, "exceeds the maximum allowed size")
}

// IsImageFormatError reports whether this error indicates an OpenAI Vision input
// rejected for an unsupported MIME type or malformed image payload (e.g. corrupt
// base64, unsupported codec, broken image_url).
func (e *APIError) IsImageFormatError() bool {
	if e == nil {
		return false
	}
	code := strings.ToLower(e.Code)
	switch code {
	case "invalid_image", "invalid_image_url", "image_url_invalid", "image_parse_error",
		"unsupported_image", "unsupported_image_format":
		return true
	}
	msg := strings.ToLower(e.Message)
	return strings.Contains(msg, "invalid image") ||
		strings.Contains(msg, "unsupported image") ||
		strings.Contains(msg, "could not parse image") ||
		strings.Contains(msg, "image_url is invalid")
}

// IsMediaSizeError reports whether this error indicates total media payload (sum
// of all images / documents) exceeded the per-request budget. Surfaced separately
// from IsImageSizeError so a multi-image request can decide whether to drop one
// image or fall back to a smaller variant.
func (e *APIError) IsMediaSizeError() bool {
	if e == nil {
		return false
	}
	code := strings.ToLower(e.Code)
	if code == "media_size_exceeded" || code == "request_too_large" {
		return true
	}
	msg := strings.ToLower(e.Message)
	return strings.Contains(msg, "request too large") ||
		strings.Contains(msg, "total media size") ||
		strings.Contains(msg, "media size limit")
}

// IsOpenAIImageSizeError reports whether the error chain contains an OpenAI
// APIError flagged as an image size violation. Provided as a free function so
// callers operating on `error` values (not concrete *APIError) can check the
// condition without manual unwrapping.
func IsOpenAIImageSizeError(err error) bool {
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	return apiErr.IsImageSizeError()
}

// IsOpenAIImageFormatError reports whether the error chain contains an OpenAI
// APIError flagged as an image format / payload violation.
func IsOpenAIImageFormatError(err error) bool {
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	return apiErr.IsImageFormatError()
}

// IsOpenAIMediaSizeError reports whether the error chain contains an OpenAI
// APIError flagged as a total media payload size violation.
func IsOpenAIMediaSizeError(err error) bool {
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	return apiErr.IsMediaSizeError()
}

// openaiErrorBody is the wire-format error response from the OpenAI API.
type openaiErrorBody struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Param   string `json:"param,omitempty"`
		Code    string `json:"code,omitempty"`
	} `json:"error"`
}

// ParseAPIError parses an OpenAI API error from an HTTP response.
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

	// Try to parse the JSON error body.
	var payload openaiErrorBody
	if len(body) > 0 {
		if jsonErr := json.Unmarshal(body, &payload); jsonErr == nil {
			err.Type = APIErrorType(payload.Error.Type)
			err.Message = payload.Error.Message
			err.Param = payload.Error.Param
			err.Code = payload.Error.Code
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
	case 408, 429, 502, 503, 504:
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
