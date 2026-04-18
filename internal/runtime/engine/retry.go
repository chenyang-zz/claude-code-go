package engine

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"net"
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

// isRetriableError classifies whether an error from Client.Stream should be retried.
func isRetriableError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()

	// Network / connection errors.
	if isNetworkError(err) {
		return true
	}

	// HTTP 5xx (including 529 overloaded).
	if strings.Contains(msg, "529") || strings.Contains(msg, "overloaded") {
		return true
	}
	if strings.Contains(msg, "500") || strings.Contains(msg, "502") || strings.Contains(msg, "503") || strings.Contains(msg, "504") {
		return true
	}

	// Rate limit.
	if strings.Contains(msg, "429") || strings.Contains(msg, "rate_limit") {
		return true
	}

	// Request timeout.
	if strings.Contains(msg, "408") || strings.Contains(msg, "timeout") {
		return true
	}

	return false
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

// tryFallback attempts to switch to the fallback model when the primary model fails.
// Returns nil if no fallback is configured or the error is not eligible for fallback.
func (e *Runtime) tryFallback(ctx context.Context, req model.Request, primaryErr error) *fallbackResult {
	if e.FallbackModel == "" {
		return nil
	}
	if !isRetriableError(primaryErr) {
		return nil
	}

	fbReq := req
	fbReq.Model = e.FallbackModel

	stream, err := e.Client.Stream(ctx, fbReq)
	if err != nil {
		return nil
	}

	return &fallbackResult{
		model:  e.FallbackModel,
		stream: stream,
	}
}

// emitEvent sends an event to the channel. The caller (runLoop goroutine) owns the channel
// and the consumer is always reading from it, so a blocking send is safe.
func emitEvent(out chan<- event.Event, evt event.Event) {
	if out == nil {
		return
	}
	out <- evt
}
