package anthropic

import (
	"errors"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/model"
)

// MapAPIError converts an Anthropic APIError into a unified ProviderError.
// It uses the error type and status code to pick the correct ProviderErrorKind.
func MapAPIError(apiErr *APIError) *model.ProviderError {
	if apiErr == nil {
		return nil
	}

	kind := classifyAnthropicError(apiErr)

	pe := model.WrapProviderError(kind, "anthropic", apiErr.Status, apiErr)
	pe.RetryAfterDuration = apiErr.RetryAfter()
	return pe
}

// MapError attempts to extract or construct a ProviderError from any error.
// If the error is already an APIError it delegates to MapAPIError.
// Otherwise it returns nil so the caller can fall back to heuristic classification.
func MapError(err error) *model.ProviderError {
	if err == nil {
		return nil
	}

	// Already a ProviderError — pass through.
	var pe *model.ProviderError
	if errors.As(err, &pe) {
		return pe
	}

	// Anthropic APIError.
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return MapAPIError(apiErr)
	}

	return nil
}

// classifyAnthropicError maps an Anthropic APIError to a ProviderErrorKind.
func classifyAnthropicError(apiErr *APIError) model.ProviderErrorKind {
	if apiErr == nil {
		return model.ProviderErrorUnknown
	}

	// Overloaded (529 or overloaded_error type) — highest priority.
	if apiErr.Status == 529 || apiErr.Type == ErrorTypeOverloaded {
		return model.ProviderErrorServerOverloaded
	}

	// Rate limit (429 or rate_limit_error type).
	if apiErr.Status == 429 || apiErr.Type == ErrorTypeRateLimit {
		return model.ProviderErrorRateLimit
	}

	// Timeout (408).
	if apiErr.Status == 408 {
		return model.ProviderErrorTimeout
	}

	// Authentication / authorization (401, 403).
	if apiErr.Status == 401 || apiErr.Status == 403 {
		if apiErr.Status == 403 && strings.Contains(apiErr.Message, "OAuth token has been revoked") {
			// OAuth token revoked is an auth error, not a quota error.
			return model.ProviderErrorAuthError
		}
		return model.ProviderErrorAuthError
	}

	// Invalid request (400, 404, 413).
	if apiErr.Status == 400 || apiErr.Status == 404 || apiErr.Status == 413 {
		return model.ProviderErrorInvalidRequest
	}

	// Server errors (500-504).
	if apiErr.Status >= 500 && apiErr.Status < 600 {
		return model.ProviderErrorServerError
	}

	// Check message content for specific patterns.
	msg := strings.ToLower(apiErr.Message)
	if strings.Contains(msg, "overloaded") {
		return model.ProviderErrorServerOverloaded
	}
	if strings.Contains(msg, "rate limit") || strings.Contains(msg, "too many requests") {
		return model.ProviderErrorRateLimit
	}
	if strings.Contains(msg, "timeout") {
		return model.ProviderErrorTimeout
	}
	if strings.Contains(msg, "credit balance is too low") {
		return model.ProviderErrorQuotaExceeded
	}

	return model.ProviderErrorUnknown
}
