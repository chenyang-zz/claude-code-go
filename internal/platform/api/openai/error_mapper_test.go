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

// TestErrorMapperInvalidImageURL covers OpenAI Vision rejections that report a
// malformed image_url payload via the structured error.code field.
func TestErrorMapperInvalidImageURL(t *testing.T) {
	apiErr := &APIError{
		Status:  400,
		Type:    ErrorTypeInvalidRequest,
		Code:    "invalid_image_url",
		Message: "Invalid image_url provided.",
	}
	pe := MapAPIError(apiErr)
	if pe.Kind != model.ProviderErrorInvalidRequest {
		t.Fatalf("Kind = %v, want ProviderErrorInvalidRequest", pe.Kind)
	}
	if !apiErr.IsImageFormatError() {
		t.Errorf("IsImageFormatError = false, want true")
	}
	if !IsOpenAIImageFormatError(apiErr) {
		t.Errorf("IsOpenAIImageFormatError(apiErr) = false, want true")
	}
	if apiErr.IsImageSizeError() {
		t.Errorf("IsImageSizeError = true, want false")
	}
}

// TestErrorMapperImageTooLarge covers OpenAI Vision rejections where a single
// image exceeds the upload size limit.
func TestErrorMapperImageTooLarge(t *testing.T) {
	apiErr := &APIError{
		Status:  400,
		Type:    ErrorTypeInvalidRequest,
		Code:    "image_too_large",
		Message: "Image too large: 25MB exceeds the maximum allowed size of 20MB.",
	}
	pe := MapAPIError(apiErr)
	if pe.Kind != model.ProviderErrorInvalidRequest {
		t.Fatalf("Kind = %v, want ProviderErrorInvalidRequest", pe.Kind)
	}
	if !apiErr.IsImageSizeError() {
		t.Errorf("IsImageSizeError = false, want true")
	}
	if !IsOpenAIImageSizeError(apiErr) {
		t.Errorf("IsOpenAIImageSizeError(apiErr) = false, want true")
	}
}

// TestErrorMapperUnsupportedImageFormat covers Vision rejections for codecs the
// model can't decode (e.g. WebP variants, animated GIFs).
func TestErrorMapperUnsupportedImageFormat(t *testing.T) {
	apiErr := &APIError{
		Status:  422,
		Type:    ErrorTypeInvalidRequest,
		Code:    "unsupported_image_format",
		Message: "Unsupported image format: image/heic.",
	}
	pe := MapAPIError(apiErr)
	if pe.Kind != model.ProviderErrorInvalidRequest {
		t.Fatalf("Kind = %v, want ProviderErrorInvalidRequest", pe.Kind)
	}
	if !apiErr.IsImageFormatError() {
		t.Errorf("IsImageFormatError = false, want true")
	}
}

// TestErrorMapperMediaSizeExceeded covers rejections where the total size of
// all attached images / documents in the request exceeds the per-call cap.
func TestErrorMapperMediaSizeExceeded(t *testing.T) {
	apiErr := &APIError{
		Status:  413,
		Type:    ErrorTypeInvalidRequest,
		Code:    "media_size_exceeded",
		Message: "Total media size exceeded the maximum allowed for one request.",
	}
	pe := MapAPIError(apiErr)
	if pe.Kind != model.ProviderErrorInvalidRequest {
		t.Fatalf("Kind = %v, want ProviderErrorInvalidRequest", pe.Kind)
	}
	if !apiErr.IsMediaSizeError() {
		t.Errorf("IsMediaSizeError = false, want true")
	}
	if !IsOpenAIMediaSizeError(apiErr) {
		t.Errorf("IsOpenAIMediaSizeError(apiErr) = false, want true")
	}
}

// TestErrorMapperImagePredicatesMessageFallback verifies the predicates fall
// back to message-based detection when the upstream Provider only fills the
// human-readable `message` field (some OpenAI-compatible vendors do not set
// the `code` field at all).
func TestErrorMapperImagePredicatesMessageFallback(t *testing.T) {
	apiErr := &APIError{
		Status:  400,
		Type:    ErrorTypeInvalidRequest,
		Message: "Image is too large to process.",
	}
	if !apiErr.IsImageSizeError() {
		t.Errorf("IsImageSizeError fallback failed for message=%q", apiErr.Message)
	}
}

// TestErrorMapperImagePredicatesNonAPIError verifies the free-function
// predicates safely return false when given non-OpenAI error chains.
func TestErrorMapperImagePredicatesNonAPIError(t *testing.T) {
	if IsOpenAIImageSizeError(errors.New("network down")) {
		t.Error("IsOpenAIImageSizeError(non-APIError) = true, want false")
	}
	if IsOpenAIImageFormatError(nil) {
		t.Error("IsOpenAIImageFormatError(nil) = true, want false")
	}
	if IsOpenAIMediaSizeError(model.NewProviderError(model.ProviderErrorRateLimit, "openai", 429, "rate")) {
		t.Error("IsOpenAIMediaSizeError(ProviderError) = true, want false")
	}
}
