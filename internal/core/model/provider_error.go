package model

import (
	"fmt"
	"time"
)

// ProviderErrorKind classifies errors from model providers into a small,
// stable set of categories that the engine can use for retry / fallback /
// circuit-breaker / health decisions.
type ProviderErrorKind string

const (
	// ProviderErrorRateLimit means the provider rejected the request because
	// rate limits were exceeded (HTTP 429).
	ProviderErrorRateLimit ProviderErrorKind = "rate_limit"
	// ProviderErrorServerOverloaded means the provider is temporarily
	// overloaded (HTTP 529 or overloaded_error in the message).
	ProviderErrorServerOverloaded ProviderErrorKind = "server_overloaded"
	// ProviderErrorServerError means an internal server error at the provider
	// (HTTP 500, 502, 503, 504).
	ProviderErrorServerError ProviderErrorKind = "server_error"
	// ProviderErrorTimeout means the request timed out before completing
	// (HTTP 408 or ETIMEDOUT).
	ProviderErrorTimeout ProviderErrorKind = "timeout"
	// ProviderErrorAuthError means authentication or authorization failed
	// (HTTP 401, 403). This is typically a configuration issue and should not
	// trigger circuit-breaker counting.
	ProviderErrorAuthError ProviderErrorKind = "auth_error"
	// ProviderErrorQuotaExceeded means the account quota has been exhausted.
	// Unlike rate_limit this is not transient; user action is required.
	ProviderErrorQuotaExceeded ProviderErrorKind = "quota_exceeded"
	// ProviderErrorInvalidRequest means the request was malformed or invalid
	// (HTTP 400, 413, 404). Retrying the same request will not help.
	ProviderErrorInvalidRequest ProviderErrorKind = "invalid_request"
	// ProviderErrorNetworkError means a low-level network failure such as
	// connection refused or reset.
	ProviderErrorNetworkError ProviderErrorKind = "network_error"
	// ProviderErrorSSLCertError means a TLS/SSL certificate validation failure.
	// This is usually caused by a corporate proxy and requires configuration.
	ProviderErrorSSLCertError ProviderErrorKind = "ssl_cert_error"
	// ProviderErrorUnknown means the error could not be classified.
	ProviderErrorUnknown ProviderErrorKind = "unknown"
)

// ProviderError wraps an error produced by a model provider with a structured
// classification. It implements error, RetryableError, and supports Unwrap.
type ProviderError struct {
	// Kind is the classified error category.
	Kind ProviderErrorKind
	// Provider identifies the provider that produced the error
	// (e.g. "anthropic", "openai", "vertex", "bedrock", "foundry").
	Provider string
	// StatusCode is the HTTP status code when available, or 0.
	StatusCode int
	// Message is a human-readable description.
	Message string
	// RetryAfterDuration is the duration the provider suggests waiting before retry.
	// Zero means the provider did not suggest a delay.
	RetryAfterDuration time.Duration
	// raw is the original error, preserved for Unwrap.
	raw error
}

// NewProviderError builds a classified provider error.
func NewProviderError(kind ProviderErrorKind, provider string, statusCode int, message string) *ProviderError {
	return &ProviderError{
		Kind:       kind,
		Provider:   provider,
		StatusCode: statusCode,
		Message:    message,
	}
}

// WrapProviderError wraps an existing error with classification metadata.
func WrapProviderError(kind ProviderErrorKind, provider string, statusCode int, raw error) *ProviderError {
	msg := ""
	if raw != nil {
		msg = raw.Error()
	}
	return &ProviderError{
		Kind:       kind,
		Provider:   provider,
		StatusCode: statusCode,
		Message:    msg,
		raw:        raw,
	}
}

// Error implements the error interface.
func (e *ProviderError) Error() string {
	if e.StatusCode > 0 {
		return fmt.Sprintf("provider %s error (%s): status=%d %s", e.Provider, e.Kind, e.StatusCode, e.Message)
	}
	return fmt.Sprintf("provider %s error (%s): %s", e.Provider, e.Kind, e.Message)
}

// Unwrap returns the original error if one was wrapped.
func (e *ProviderError) Unwrap() error {
	return e.raw
}

// IsRetryable reports whether this error kind warrants a retry.
func (e *ProviderError) IsRetryable() bool {
	switch e.Kind {
	case ProviderErrorRateLimit,
		ProviderErrorServerOverloaded,
		ProviderErrorServerError,
		ProviderErrorTimeout,
		ProviderErrorNetworkError:
		return true
	case ProviderErrorAuthError:
		// Auth errors are retryable *once* after credential refresh,
		// but the engine should handle refresh separately. For the
		// purpose of exponential-backoff retry we treat them as retryable
		// so the retry loop can attempt a fresh credential fetch.
		return true
	case ProviderErrorQuotaExceeded,
		ProviderErrorInvalidRequest,
		ProviderErrorSSLCertError,
		ProviderErrorUnknown:
		return false
	}
	return false
}

// RetryAfter implements RetryableError. It returns the provider-suggested
// wait duration, or zero when none was given.
func (e *ProviderError) RetryAfter() time.Duration {
	return e.RetryAfterDuration
}

// ShouldTriggerCircuitBreaker reports whether this error should count toward
// the circuit-breaker failure threshold. Auth and quota errors are excluded
// because they are typically configuration issues that will not be resolved
// by simply waiting.
func (e *ProviderError) ShouldTriggerCircuitBreaker() bool {
	switch e.Kind {
	case ProviderErrorRateLimit,
		ProviderErrorServerOverloaded,
		ProviderErrorServerError,
		ProviderErrorTimeout,
		ProviderErrorNetworkError:
		return true
	case ProviderErrorAuthError,
		ProviderErrorQuotaExceeded,
		ProviderErrorInvalidRequest,
		ProviderErrorSSLCertError,
		ProviderErrorUnknown:
		return false
	}
	return false
}

// HealthImpact returns the health status that should be reported when this
// error is encountered during a health probe.
func (e *ProviderError) HealthImpact() HealthStatus {
	switch e.Kind {
	case ProviderErrorRateLimit,
		ProviderErrorServerOverloaded,
		ProviderErrorTimeout,
		ProviderErrorNetworkError:
		return HealthStatusDegraded
	case ProviderErrorServerError,
		ProviderErrorAuthError,
		ProviderErrorQuotaExceeded,
		ProviderErrorSSLCertError:
		return HealthStatusUnhealthy
	case ProviderErrorInvalidRequest:
		// A 400 from the provider usually means the probe request itself
		// was rejected for a non-health reason (e.g. model not found).
		return HealthStatusHealthy
	case ProviderErrorUnknown:
		return HealthStatusUnknown
	}
	return HealthStatusUnknown
}

// ProviderErrorKindForRetryable maps a retriable boolean to a more precise
// kind when the only information available is "should retry". It is a
// fallback used by the engine when an error does not carry a ProviderError.
func ProviderErrorKindForRetryable(isRetryable bool, statusCode int) ProviderErrorKind {
	if !isRetryable {
		return ProviderErrorUnknown
	}
	switch statusCode {
	case 429:
		return ProviderErrorRateLimit
	case 529:
		return ProviderErrorServerOverloaded
	case 408:
		return ProviderErrorTimeout
	case 500, 502, 503, 504:
		return ProviderErrorServerError
	case 401, 403:
		return ProviderErrorAuthError
	default:
		return ProviderErrorServerError
	}
}
