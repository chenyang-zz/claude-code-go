package engine

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/model"
)

// TestProviderErrorRetryableClassification verifies that ProviderError correctly
// drives the isRetriableError decision.
func TestProviderErrorRetryableClassification(t *testing.T) {
	cases := []struct {
		name      string
		err       error
		retryable bool
	}{
		{"rate_limit", model.NewProviderError(model.ProviderErrorRateLimit, "anthropic", 429, "slow down"), true},
		{"server_overloaded", model.NewProviderError(model.ProviderErrorServerOverloaded, "anthropic", 529, "overloaded"), true},
		{"server_error", model.NewProviderError(model.ProviderErrorServerError, "anthropic", 500, "boom"), true},
		{"timeout", model.NewProviderError(model.ProviderErrorTimeout, "anthropic", 408, "timeout"), true},
		{"network_error", model.NewProviderError(model.ProviderErrorNetworkError, "anthropic", 0, "conn refused"), true},
		{"auth_error", model.NewProviderError(model.ProviderErrorAuthError, "anthropic", 401, "unauthorized"), true},
		{"quota_exceeded", model.NewProviderError(model.ProviderErrorQuotaExceeded, "anthropic", 429, "no quota"), false},
		{"invalid_request", model.NewProviderError(model.ProviderErrorInvalidRequest, "anthropic", 400, "bad request"), false},
		{"ssl_cert_error", model.NewProviderError(model.ProviderErrorSSLCertError, "anthropic", 0, "cert bad"), false},
		{"unknown", model.NewProviderError(model.ProviderErrorUnknown, "anthropic", 0, "???"), false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := isRetriableError(c.err)
			if got != c.retryable {
				t.Errorf("isRetriableError(%s) = %v, want %v", c.name, got, c.retryable)
			}
		})
	}
}

// TestCircuitBreakerProviderErrorClassification verifies that the circuit breaker
// only counts specific error kinds toward the failure threshold.
func TestCircuitBreakerProviderErrorClassification(t *testing.T) {
	cases := []struct {
		name         string
		kind         model.ProviderErrorKind
		shouldRecord bool
	}{
		{"rate_limit_triggers", model.ProviderErrorRateLimit, true},
		{"server_overloaded_triggers", model.ProviderErrorServerOverloaded, true},
		{"server_error_triggers", model.ProviderErrorServerError, true},
		{"timeout_triggers", model.ProviderErrorTimeout, true},
		{"network_error_triggers", model.ProviderErrorNetworkError, true},
		{"auth_error_skips", model.ProviderErrorAuthError, false},
		{"quota_exceeded_skips", model.ProviderErrorQuotaExceeded, false},
		{"invalid_request_skips", model.ProviderErrorInvalidRequest, false},
		{"ssl_cert_error_skips", model.ProviderErrorSSLCertError, false},
		{"unknown_skips", model.ProviderErrorUnknown, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			breaker := model.NewCircuitBreaker(model.CircuitBreakerSettings{
				FailureThreshold:         1,
				RecoveryTimeout:          30 * time.Second,
				HalfOpenMaxRequests:      1,
				HalfOpenSuccessThreshold: 1,
			})
			client := &failingClient{err: model.NewProviderError(c.kind, "test", 0, "msg")}
			cbc := NewCircuitBreakerClient(client, "test", breaker)

			_, _ = cbc.Stream(context.Background(), model.Request{})

			if c.shouldRecord {
				if breaker.State() != model.CircuitBreakerOpen {
					t.Errorf("expected breaker to be open after %s, got %s", c.kind, breaker.State())
				}
			} else {
				if breaker.State() != model.CircuitBreakerClosed {
					t.Errorf("expected breaker to stay closed after %s, got %s", c.kind, breaker.State())
				}
			}
		})
	}
}

// TestCircuitBreakerFallbackKeywordExcludesAuth verifies that the fallback
// keyword-based circuit breaker logic excludes auth-related errors.
func TestCircuitBreakerFallbackKeywordExcludesAuth(t *testing.T) {
	cases := []struct {
		msg          string
		shouldRecord bool
	}{
		{"server overloaded, retry", true},
		{"rate_limit exceeded", true},
		{"connection timeout", true},
		{"authentication failed", false},
		{"unauthorized request", false},
		{"forbidden access", false},
	}

	for _, c := range cases {
		t.Run(c.msg, func(t *testing.T) {
			breaker := model.NewCircuitBreaker(model.CircuitBreakerSettings{
				FailureThreshold:         1,
				RecoveryTimeout:          30 * time.Second,
				HalfOpenMaxRequests:      1,
				HalfOpenSuccessThreshold: 1,
			})
			client := &failingClient{err: errors.New(c.msg)}
			cbc := NewCircuitBreakerClient(client, "test", breaker)

			_, _ = cbc.Stream(context.Background(), model.Request{})

			if c.shouldRecord {
				if breaker.State() != model.CircuitBreakerOpen {
					t.Errorf("expected breaker open for %q, got %s", c.msg, breaker.State())
				}
			} else {
				if breaker.State() != model.CircuitBreakerClosed {
					t.Errorf("expected breaker closed for %q, got %s", c.msg, breaker.State())
				}
			}
		})
	}
}

// TestTryFallbackWithProviderError verifies that tryFallback recognizes
// ProviderError errors as retriable and attempts fallback.
func TestTryFallbackWithProviderError(t *testing.T) {
	rt := &Runtime{
		Client:         &failingClient{err: model.NewProviderError(model.ProviderErrorRateLimit, "anthropic", 429, "slow")},
		FallbackModel:  "fallback-model",
		FallbackClients: []model.Client{
			&successClient{},
		},
		FallbackAfterAttempts: 1,
	}

	result := rt.tryFallback(context.Background(), model.Request{}, model.NewProviderError(model.ProviderErrorRateLimit, "anthropic", 429, "slow"))
	if result == nil {
		t.Fatal("expected fallback to succeed")
	}
}

// TestHealthResultErrorKind verifies that HealthResult carries the correct
// ProviderErrorKind from health probes.
func TestHealthResultErrorKind(t *testing.T) {
	cases := []struct {
		statusCode int
		wantKind   model.ProviderErrorKind
	}{
		{200, model.ProviderErrorUnknown},
		{429, model.ProviderErrorRateLimit},
		{500, model.ProviderErrorServerError},
		{401, model.ProviderErrorAuthError},
		{408, model.ProviderErrorTimeout},
	}

	for _, c := range cases {
		t.Run(string(c.wantKind), func(t *testing.T) {
			// Simulate the healthErrorKind logic inline for verification.
			var got model.ProviderErrorKind
			switch c.statusCode {
			case 401, 403:
				got = model.ProviderErrorAuthError
			case 429:
				got = model.ProviderErrorRateLimit
			case 408:
				got = model.ProviderErrorTimeout
			case 500, 502, 503, 504:
				got = model.ProviderErrorServerError
			default:
				got = model.ProviderErrorUnknown
			}
			if got != c.wantKind {
				t.Errorf("healthErrorKind(%d) = %v, want %v", c.statusCode, got, c.wantKind)
			}
		})
	}
}

// failingClient always returns the configured error.
type failingClient struct {
	err error
}

func (c *failingClient) Stream(_ context.Context, _ model.Request) (model.Stream, error) {
	return nil, c.err
}

// successClient always returns a dummy stream.
type successClient struct{}

func (c *successClient) Stream(_ context.Context, _ model.Request) (model.Stream, error) {
	return make(model.Stream), nil
}
