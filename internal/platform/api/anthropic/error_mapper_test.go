package anthropic

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
			name:       "overloaded 529",
			apiErr:     &APIError{Status: 529, Type: ErrorTypeOverloaded, Message: "overloaded"},
			wantKind:   model.ProviderErrorServerOverloaded,
			wantStatus: 529,
		},
		{
			name:       "server error 500",
			apiErr:     &APIError{Status: 500, Type: ErrorTypeAPIError, Message: "internal"},
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
			apiErr:     &APIError{Status: 403, Type: ErrorTypePermission, Message: "forbidden"},
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
			apiErr:     &APIError{Status: 404, Type: ErrorTypeNotFound, Message: "not found"},
			wantKind:   model.ProviderErrorInvalidRequest,
			wantStatus: 404,
		},
		{
			name:       "message pattern overloaded",
			apiErr:     &APIError{Status: 0, Message: "The API is overloaded, please retry"},
			wantKind:   model.ProviderErrorServerOverloaded,
			wantStatus: 0,
		},
		{
			name:       "message pattern rate limit",
			apiErr:     &APIError{Status: 0, Message: "Rate limit exceeded"},
			wantKind:   model.ProviderErrorRateLimit,
			wantStatus: 0,
		},
		{
			name:       "credit balance",
			apiErr:     &APIError{Status: 429, Message: "Credit balance is too low"},
			wantKind:   model.ProviderErrorRateLimit,
			wantStatus: 429,
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
			if got.Provider != "anthropic" {
				t.Errorf("Provider = %v, want anthropic", got.Provider)
			}
		})
	}
}

func TestMapAPIErrorRetryAfter(t *testing.T) {
	headers := http.Header{}
	headers.Set("retry-after", "5")
	apiErr := &APIError{
		Status:  429,
		Type:    ErrorTypeRateLimit,
		Message: "slow down",
		Headers: headers,
	}

	got := MapAPIError(apiErr)
	if got.RetryAfterDuration != 5*time.Second {
		t.Errorf("RetryAfterDuration = %v, want 5s", got.RetryAfterDuration)
	}
}

func TestMapErrorPassthrough(t *testing.T) {
	// ProviderError should pass through unchanged.
	orig := model.NewProviderError(model.ProviderErrorRateLimit, "anthropic", 429, "test")
	got := MapError(orig)
	if got != orig {
		t.Error("expected MapError to pass through *ProviderError unchanged")
	}
}

func TestMapErrorWithAPIError(t *testing.T) {
	apiErr := &APIError{Status: 500, Type: ErrorTypeAPIError, Message: "boom"}
	got := MapError(apiErr)
	if got == nil {
		t.Fatal("expected MapError to extract ProviderError from APIError")
	}
	if got.Kind != model.ProviderErrorServerError {
		t.Errorf("Kind = %v, want server_error", got.Kind)
	}
}

func TestMapErrorUnmapped(t *testing.T) {
	err := errors.New("some random error")
	got := MapError(err)
	if got != nil {
		t.Errorf("MapError(random) = %v, want nil", got)
	}
}
