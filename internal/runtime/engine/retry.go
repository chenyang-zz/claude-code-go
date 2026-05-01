package engine

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"net"
	"regexp"
	"strings"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/event"
	"github.com/sheepzhao/claude-code-go/internal/core/model"
)

// RetryPolicy controls how the engine retries transient provider errors.
type RetryPolicy struct {
	// MaxAttempts is the maximum number of retries before giving up. Zero means no retry.
	MaxAttempts int
	// InitialBackoff is the base delay for the first retry.
	InitialBackoff time.Duration
	// MaxBackoff caps the exponential backoff duration.
	MaxBackoff time.Duration
}

// DefaultRetryPolicy returns a sensible default retry policy.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxAttempts:    3,
		InitialBackoff: 500 * time.Millisecond,
		MaxBackoff:     30 * time.Second,
	}
}

// backoffDuration computes the delay before the given attempt (1-based) with jitter.
func (p RetryPolicy) backoffDuration(attempt int) time.Duration {
	base := p.InitialBackoff
	if base <= 0 {
		base = 500 * time.Millisecond
	}
	maxBackoff := p.MaxBackoff
	if maxBackoff <= 0 {
		maxBackoff = 30 * time.Second
	}

	exp := time.Duration(float64(base) * math.Pow(2, float64(attempt-1)))
	if exp > maxBackoff {
		exp = maxBackoff
	}

	// Add up to 25% jitter.
	jitter := time.Duration(rand.Int63n(int64(float64(exp) * 0.25)))
	return exp + jitter
}

// httpStatusRe matches standalone 3-digit HTTP status codes in error messages.
// Uses word boundaries to avoid false positives like "250000" matching "500".
var httpStatusRe = regexp.MustCompile(`\b(529|500|502|503|504|429|408)\b`)

// isRetriableError classifies whether an error from Client.Stream should be retried.
func isRetriableError(err error) bool {
	if err == nil {
		return false
	}

	// Check if the error implements the RetryableError interface.
	var retryable model.RetryableError
	if errors.As(err, &retryable) {
		return retryable.IsRetryable()
	}

	msg := err.Error()

	// Network / connection errors.
	if isNetworkError(err) {
		return true
	}

	// HTTP status codes: 5xx (including 529 overloaded), 429 (rate limit), 408 (timeout).
	if httpStatusRe.MatchString(msg) {
		return true
	}

	// Keyword-based fallback for rate limit and timeout messages.
	if strings.Contains(msg, "overloaded") || strings.Contains(msg, "rate_limit") || strings.Contains(msg, "timeout") {
		return true
	}

	// OpenAI-specific retryable error messages (fallback when the error is not a structured APIError).
	lower := strings.ToLower(msg)
	if strings.Contains(lower, "server_error") || strings.Contains(lower, "temporary error") || strings.Contains(lower, "over capacity") {
		return true
	}

	return false
}

// isPromptTooLongError detects API errors indicating the conversation prompt exceeds
// the model's context window. Covers Anthropic ("prompt is too long"),
// OpenAI ("context_length_exceeded"), and the streaming stop-reason variant
// ("context_window_exceeded" / "model_context_window_exceeded").
func isPromptTooLongError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "prompt is too long") ||
		strings.Contains(msg, "context_length_exceeded") ||
		strings.Contains(msg, "model_context_window_exceeded") ||
		strings.Contains(msg, "context_window_exceeded")
}

func isNetworkError(err error) bool {
	var netErr net.Error
	if strings.Contains(err.Error(), "connection refused") ||
		strings.Contains(err.Error(), "connection reset") ||
		strings.Contains(err.Error(), "ECONNREFUSED") ||
		strings.Contains(err.Error(), "ECONNRESET") ||
		strings.Contains(err.Error(), "EPIPE") {
		return true
	}
	// Also match standard net.Error interface.
	if errors.As(err, &netErr) {
		return true
	}
	return false
}

// fallbackResult holds the result of a successful fallback attempt.
type fallbackResult struct {
	model  string
	stream model.Stream
}

// tryFallback attempts to switch to the fallback model or alternate provider
// client when the primary model fails. Returns nil if no fallback is configured
// or the error is not eligible for fallback.
func (e *Runtime) tryFallback(ctx context.Context, req model.Request, primaryErr error) *fallbackResult {
	if !isRetriableError(primaryErr) {
		return nil
	}

	// 1. Try the in-provider fallback model first (existing behaviour).
	if e.FallbackModel != "" {
		fbReq := req
		fbReq.Model = e.FallbackModel
		stream, err := e.Client.Stream(ctx, fbReq)
		if err == nil {
			return &fallbackResult{
				model:  e.FallbackModel,
				stream: stream,
			}
		}
	}

	// 2. Try cross-provider fallback clients in order.
	for i, client := range e.FallbackClients {
		if client == nil {
			continue
		}
		stream, err := client.Stream(ctx, req)
		if err == nil {
			return &fallbackResult{
				model:  fmt.Sprintf("fallback-client-%d", i),
				stream: stream,
			}
		}
	}

	return nil
}

// shouldFallbackAfterAttempts reports whether fallback should be triggered after
// the given attempt count, respecting the FallbackAfterAttempts setting.
func (e *Runtime) shouldFallbackAfterAttempts(attempt int) bool {
	if e.FallbackAfterAttempts <= 0 {
		return false
	}
	return attempt >= e.FallbackAfterAttempts
}

// emitEvent sends an event to the channel. The caller (runLoop goroutine) owns the channel
// and the consumer is always reading from it, so a blocking send is safe.
func emitEvent(out chan<- event.Event, evt event.Event) {
	if out == nil {
		return
	}
	out <- evt
}
