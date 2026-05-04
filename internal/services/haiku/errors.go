package haiku

import (
	"context"
	"errors"
	"net"

	"github.com/sheepzhao/claude-code-go/internal/platform/api/anthropic"
)

// ErrHaikuDisabled is returned by Service.Query when FlagHaikuQuery is
// explicitly set to "0" / "false". The error allows callers to distinguish a
// disabled helper from a transient API failure.
var ErrHaikuDisabled = errors.New("haiku: feature flag disabled")

// ErrClientUnavailable is returned by Service.Query when the underlying
// model client has not been wired into the service (typically because
// bootstrap chose a non-Anthropic provider).
var ErrClientUnavailable = errors.New("haiku: model client unavailable")

// IsRateLimit reports whether err is a rate-limit response from the
// Anthropic API. Non-Anthropic errors return false.
func IsRateLimit(err error) bool {
	if err == nil {
		return false
	}
	var apiErr *anthropic.APIError
	if errors.As(err, &apiErr) {
		return apiErr.IsRateLimit()
	}
	return false
}

// IsNetwork reports whether err is a transient network failure (DNS, dial,
// timeout, context deadline) rather than an Anthropic API response.
func IsNetwork(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	// Anthropic API errors carry HTTP status codes and should not be
	// classified as transient network failures.
	var apiErr *anthropic.APIError
	if errors.As(err, &apiErr) {
		return false
	}
	return false
}

// IsAPIError reports whether err is a structured Anthropic API error
// (any HTTP status that the server returned with a parseable error body).
func IsAPIError(err error) bool {
	if err == nil {
		return false
	}
	var apiErr *anthropic.APIError
	return errors.As(err, &apiErr)
}
