package model

import (
	"errors"
	"testing"
	"time"
)

func TestProviderErrorNew(t *testing.T) {
	err := NewProviderError(ProviderErrorRateLimit, "anthropic", 429, "too many requests")
	if err.Kind != ProviderErrorRateLimit {
		t.Errorf("Kind = %v, want %v", err.Kind, ProviderErrorRateLimit)
	}
	if err.Provider != "anthropic" {
		t.Errorf("Provider = %v, want anthropic", err.Provider)
	}
	if err.StatusCode != 429 {
		t.Errorf("StatusCode = %d, want 429", err.StatusCode)
	}
	if err.Message != "too many requests" {
		t.Errorf("Message = %v, want 'too many requests'", err.Message)
	}
}

func TestProviderErrorWrap(t *testing.T) {
	orig := errors.New("connection refused")
	err := WrapProviderError(ProviderErrorNetworkError, "openai", 0, orig)
	if err.Message != "connection refused" {
		t.Errorf("Message = %v, want 'connection refused'", err.Message)
	}
	if !errors.Is(err, orig) {
		t.Error("expected errors.Is to match the wrapped error")
	}
	if errors.Unwrap(err) != orig {
		t.Error("expected Unwrap to return the original error")
	}
}

func TestProviderErrorErrorString(t *testing.T) {
	cases := []struct {
		name     string
		err      *ProviderError
		wantSub  string
	}{
		{
			name:    "with status",
			err:     NewProviderError(ProviderErrorRateLimit, "anthropic", 429, "too many"),
			wantSub: "status=429",
		},
		{
			name:    "without status",
			err:     NewProviderError(ProviderErrorNetworkError, "bedrock", 0, "timeout"),
			wantSub: "network_error",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := c.err.Error()
			if !contains(got, c.wantSub) {
				t.Errorf("Error() = %q, want substring %q", got, c.wantSub)
			}
		})
	}
}

func TestProviderErrorIsRetryable(t *testing.T) {
	cases := []struct {
		kind     ProviderErrorKind
		retryable bool
	}{
		{ProviderErrorRateLimit, true},
		{ProviderErrorServerOverloaded, true},
		{ProviderErrorServerError, true},
		{ProviderErrorTimeout, true},
		{ProviderErrorNetworkError, true},
		{ProviderErrorAuthError, true},
		{ProviderErrorQuotaExceeded, false},
		{ProviderErrorInvalidRequest, false},
		{ProviderErrorSSLCertError, false},
		{ProviderErrorUnknown, false},
	}
	for _, c := range cases {
		t.Run(string(c.kind), func(t *testing.T) {
			err := NewProviderError(c.kind, "test", 0, "msg")
			if got := err.IsRetryable(); got != c.retryable {
				t.Errorf("IsRetryable() = %v, want %v", got, c.retryable)
			}
		})
	}
}

func TestProviderErrorShouldTriggerCircuitBreaker(t *testing.T) {
	cases := []struct {
		kind      ProviderErrorKind
		shouldTrip bool
	}{
		{ProviderErrorRateLimit, true},
		{ProviderErrorServerOverloaded, true},
		{ProviderErrorServerError, true},
		{ProviderErrorTimeout, true},
		{ProviderErrorNetworkError, true},
		{ProviderErrorAuthError, false},
		{ProviderErrorQuotaExceeded, false},
		{ProviderErrorInvalidRequest, false},
		{ProviderErrorSSLCertError, false},
		{ProviderErrorUnknown, false},
	}
	for _, c := range cases {
		t.Run(string(c.kind), func(t *testing.T) {
			err := NewProviderError(c.kind, "test", 0, "msg")
			if got := err.ShouldTriggerCircuitBreaker(); got != c.shouldTrip {
				t.Errorf("ShouldTriggerCircuitBreaker() = %v, want %v", got, c.shouldTrip)
			}
		})
	}
}

func TestProviderErrorHealthImpact(t *testing.T) {
	cases := []struct {
		kind  ProviderErrorKind
		want  HealthStatus
	}{
		{ProviderErrorRateLimit, HealthStatusDegraded},
		{ProviderErrorServerOverloaded, HealthStatusDegraded},
		{ProviderErrorTimeout, HealthStatusDegraded},
		{ProviderErrorNetworkError, HealthStatusDegraded},
		{ProviderErrorServerError, HealthStatusUnhealthy},
		{ProviderErrorAuthError, HealthStatusUnhealthy},
		{ProviderErrorQuotaExceeded, HealthStatusUnhealthy},
		{ProviderErrorSSLCertError, HealthStatusUnhealthy},
		{ProviderErrorInvalidRequest, HealthStatusHealthy},
		{ProviderErrorUnknown, HealthStatusUnknown},
	}
	for _, c := range cases {
		t.Run(string(c.kind), func(t *testing.T) {
			err := NewProviderError(c.kind, "test", 0, "msg")
			if got := err.HealthImpact(); got != c.want {
				t.Errorf("HealthImpact() = %v, want %v", got, c.want)
			}
		})
	}
}

func TestProviderErrorRetryAfter(t *testing.T) {
	err := NewProviderError(ProviderErrorRateLimit, "anthropic", 429, "slow down")
	err.RetryAfterDuration = 5 * time.Second
	if got := err.RetryAfter(); got != 5*time.Second {
		t.Errorf("RetryAfter() = %v, want 5s", got)
	}
}

func TestProviderErrorKindForRetryable(t *testing.T) {
	cases := []struct {
		isRetryable bool
		statusCode  int
		want        ProviderErrorKind
	}{
		{false, 429, ProviderErrorUnknown},
		{true, 429, ProviderErrorRateLimit},
		{true, 529, ProviderErrorServerOverloaded},
		{true, 408, ProviderErrorTimeout},
		{true, 500, ProviderErrorServerError},
		{true, 401, ProviderErrorAuthError},
		{true, 403, ProviderErrorAuthError},
		{true, 0, ProviderErrorServerError},
	}
	for _, c := range cases {
		t.Run(string(c.want), func(t *testing.T) {
			got := ProviderErrorKindForRetryable(c.isRetryable, c.statusCode)
			if got != c.want {
				t.Errorf("ProviderErrorKindForRetryable(%v, %d) = %v, want %v", c.isRetryable, c.statusCode, got, c.want)
			}
		})
	}
}

func TestProviderErrorImplementsRetryableError(t *testing.T) {
	var _ RetryableError = (*ProviderError)(nil)
}

func TestProviderErrorErrorsAs(t *testing.T) {
	orig := errors.New("original")
	err := WrapProviderError(ProviderErrorServerError, "anthropic", 500, orig)

	var pe *ProviderError
	if !errors.As(err, &pe) {
		t.Fatal("expected errors.As to match *ProviderError")
	}
	if pe.Kind != ProviderErrorServerError {
		t.Errorf("Kind = %v, want server_error", pe.Kind)
	}
}

func contains(s, sub string) bool {
	return len(sub) <= len(s) && (s == sub || len(s) > 0 && containsSub(s, sub))
}

func containsSub(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
