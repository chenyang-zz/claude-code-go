package openai

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/model"
)

func TestMapAPIError(t *testing.T) {
	cases := []struct {
		name       string
		apiErr     *APIError
		wantKind   model.ProviderErrorKind
		wantStatus int
	}{
		{
			name:       "rate limit 429",
			apiErr:     &APIError{Status: 429, Type: ErrorTypeRateLimit, Message: "too many"},
			wantKind:   model.ProviderErrorRateLimit,
			wantStatus: 429,
		},
		{
			name:       "insufficient_quota",
			apiErr:     &APIError{Status: 429, Type: ErrorTypePermission, Code: "insufficient_quota", Message: "quota"},
			wantKind:   model.ProviderErrorQuotaExceeded,
			wantStatus: 429,
		},
		{
			name:       "server error 500",
			apiErr:     &APIError{Status: 500, Type: ErrorTypeServerError, Message: "internal"},
			wantKind:   model.ProviderErrorServerError,
			wantStatus: 500,
		},
		{
			name:       "timeout 408",
			apiErr:     &APIError{Status: 408, Message: "timeout"},
			wantKind:   model.ProviderErrorTimeout,
			wantStatus: 408,
		},
		{
			name:       "auth 401",
			apiErr:     &APIError{Status: 401, Type: ErrorTypeAuthentication, Message: "unauthorized"},
			wantKind:   model.ProviderErrorAuthError,
			wantStatus: 401,
		},
		{
			name:       "auth 403",
			apiErr:     &APIError{Status: 403, Message: "forbidden"},
			wantKind:   model.ProviderErrorAuthError,
			wantStatus: 403,
		},
		{
			name:       "invalid request 400",
			apiErr:     &APIError{Status: 400, Type: ErrorTypeInvalidRequest, Message: "bad request"},
			wantKind:   model.ProviderErrorInvalidRequest,
			wantStatus: 400,
		},
		{
			name:       "not found 404",
			apiErr:     &APIError{Status: 404, Message: "not found"},
			wantKind:   model.ProviderErrorInvalidRequest,
			wantStatus: 404,
		},
		{
			name:       "code insufficient_quota",
			apiErr:     &APIError{Status: 429, Code: "insufficient_quota", Message: "no quota"},
			wantKind:   model.ProviderErrorQuotaExceeded,
			wantStatus: 429,
		},
		{
			name:       "code rate_limit_exceeded",
			apiErr:     &APIError{Status: 429, Code: "rate_limit_exceeded", Message: "slow"},
			wantKind:   model.ProviderErrorRateLimit,
			wantStatus: 429,
		},
		{
			name:       "message pattern over capacity",
			apiErr:     &APIError{Status: 503, Message: "Over capacity"},
			wantKind:   model.ProviderErrorServerError,
			wantStatus: 503,
		},
		{
			name:       "nil input",
			apiErr:     nil,
			wantKind:   model.ProviderErrorUnknown,
			wantStatus: 0,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := MapAPIError(c.apiErr)
			if c.apiErr == nil {
				if got != nil {
					t.Fatalf("MapAPIError(nil) = %v, want nil", got)
				}
				return
			}
			if got.Kind != c.wantKind {
				t.Errorf("Kind = %v, want %v", got.Kind, c.wantKind)
			}
			if got.StatusCode != c.wantStatus {
				t.Errorf("StatusCode = %d, want %d", got.StatusCode, c.wantStatus)
			}
			if got.Provider != "openai" {
				t.Errorf("Provider = %v, want openai", got.Provider)
			}
		})
	}
}

func TestMapAPIErrorRetryAfter(t *testing.T) {
	headers := http.Header{}
	headers.Set("retry-after", "10")
	apiErr := &APIError{
		Status:  429,
		Type:    ErrorTypeRateLimit,
		Message: "slow down",
		Headers: headers,
	}

	got := MapAPIError(apiErr)
	if got.RetryAfterDuration != 10*time.Second {
		t.Errorf("RetryAfterDuration = %v, want 10s", got.RetryAfterDuration)
	}
}

func TestMapErrorPassthrough(t *testing.T) {
	orig := model.NewProviderError(model.ProviderErrorRateLimit, "openai", 429, "test")
	got := MapError(orig)
	if got != orig {
		t.Error("expected MapError to pass through *ProviderError unchanged")
	}
}

func TestMapErrorUnmapped(t *testing.T) {
	err := errors.New("some random error")
	got := MapError(err)
	if got != nil {
		t.Errorf("MapError(random) = %v, want nil", got)
	}
}
