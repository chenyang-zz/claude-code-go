package openai

import (
	"errors"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/model"
)

// MapAPIError converts an OpenAI-compatible APIError into a unified ProviderError.
// It uses the error type, code, and status code to pick the correct ProviderErrorKind.
func MapAPIError(apiErr *APIError) *model.ProviderError {
	if apiErr == nil {
		return nil
	}

	kind := classifyOpenAIError(apiErr)

	pe := model.WrapProviderError(kind, "openai", apiErr.Status, apiErr)
	pe.RetryAfterDuration = apiErr.RetryAfter()
	return pe
}

// MapError attempts to extract or construct a ProviderError from any error.
// If the error is already an APIError it delegates to MapAPIError.
func MapError(err error) *model.ProviderError {
	if err == nil {
		return nil
	}

	// Already a ProviderError — pass through.
	var pe *model.ProviderError
	if errors.As(err, &pe) {
		return pe
	}

	// OpenAI APIError.
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return MapAPIError(apiErr)
	}

	return nil
}

// classifyOpenAIError maps an OpenAI APIError to a ProviderErrorKind.
func classifyOpenAIError(apiErr *APIError) model.ProviderErrorKind {
	if apiErr == nil {
		return model.ProviderErrorUnknown
	}

	// Check code field first — it is the most specific signal.
	code := strings.ToLower(apiErr.Code)
	switch code {
	case "insufficient_quota":
		return model.ProviderErrorQuotaExceeded
	case "rate_limit_exceeded":
		return model.ProviderErrorRateLimit
	case "invalid_api_key":
		return model.ProviderErrorAuthError
	case "context_length_exceeded":
		return model.ProviderErrorInvalidRequest
	}

	// Check specific error types — they may have ambiguous status codes
	// (e.g. insufficient_quota can come back as 429 on some providers).
	if apiErr.Type == ErrorTypePermission {
		return model.ProviderErrorQuotaExceeded
	}
	if apiErr.Type == ErrorTypeRateLimit {
		return model.ProviderErrorRateLimit
	}

	// Rate limit by status code.
	if apiErr.Status == 429 {
		return model.ProviderErrorRateLimit
	}

	// Timeout (408).
	if apiErr.Status == 408 {
		return model.ProviderErrorTimeout
	}

	// Authentication (401).
	if apiErr.Status == 401 || apiErr.Type == ErrorTypeAuthentication {
		return model.ProviderErrorAuthError
	}

	// Permission / forbidden (403).
	if apiErr.Status == 403 {
		return model.ProviderErrorAuthError
	}

	// Invalid request (400, 404, 422).
	if apiErr.Status == 400 || apiErr.Status == 404 || apiErr.Status == 422 {
		return model.ProviderErrorInvalidRequest
	}

	// Server errors (500-504).
	if apiErr.Status >= 500 && apiErr.Status < 600 {
		return model.ProviderErrorServerError
	}

	// Check message content for specific patterns.
	msg := strings.ToLower(apiErr.Message)
	if strings.Contains(msg, "over capacity") || strings.Contains(msg, "server_error") {
		return model.ProviderErrorServerError
	}
	if strings.Contains(msg, "rate limit") || strings.Contains(msg, "too many requests") {
		return model.ProviderErrorRateLimit
	}
	if strings.Contains(msg, "timeout") {
		return model.ProviderErrorTimeout
	}
	if strings.Contains(msg, "invalid api key") || strings.Contains(msg, "incorrect api key") {
		return model.ProviderErrorAuthError
	}
	if strings.Contains(msg, "quota") || strings.Contains(msg, "billing") {
		return model.ProviderErrorQuotaExceeded
	}

	return model.ProviderErrorUnknown
}
