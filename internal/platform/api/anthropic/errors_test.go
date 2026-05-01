package anthropic

import (
	"net/http"
	"testing"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
)

func TestAPIErrorIsRetryable(t *testing.T) {
	tests := []struct {
		name string
		err  *APIError
		want bool
	}{
		{
			name: "529 overloaded status",
			err:  &APIError{Status: 529, Type: ErrorTypeOverloaded, Message: "overloaded"},
			want: true,
		},
		{
			name: "overloaded_error type without 529",
			err:  &APIError{Status: 500, Type: ErrorTypeOverloaded, Message: "overloaded"},
			want: true,
		},
		{
			name: "429 rate limit status",
			err:  &APIError{Status: 429, Type: ErrorTypeRateLimit, Message: "rate limited"},
			want: true,
		},
		{
			name: "rate_limit_error type",
			err:  &APIError{Status: 400, Type: ErrorTypeRateLimit, Message: "rate limited"},
			want: true,
		},
		{
			name: "408 request timeout",
			err:  &APIError{Status: 408, Message: "timeout"},
			want: true,
		},
		{
			name: "409 lock timeout",
			err:  &APIError{Status: 409, Message: "conflict"},
			want: true,
		},
		{
			name: "401 authentication",
			err:  &APIError{Status: 401, Type: ErrorTypeAuthentication, Message: "unauthorized"},
			want: true,
		},
		{
			name: "403 OAuth token revoked",
			err:  &APIError{Status: 403, Message: "OAuth token has been revoked"},
			want: true,
		},
		{
			name: "403 other permission error",
			err:  &APIError{Status: 403, Type: ErrorTypePermission, Message: "forbidden"},
			want: false,
		},
		{
			name: "500 internal server error",
			err:  &APIError{Status: 500, Type: ErrorTypeAPIError, Message: "internal error"},
			want: true,
		},
		{
			name: "502 bad gateway",
			err:  &APIError{Status: 502, Message: "bad gateway"},
			want: true,
		},
		{
			name: "503 service unavailable",
			err:  &APIError{Status: 503, Message: "service unavailable"},
			want: true,
		},
		{
			name: "504 gateway timeout",
			err:  &APIError{Status: 504, Message: "gateway timeout"},
			want: true,
		},
		{
			name: "400 bad request",
			err:  &APIError{Status: 400, Type: ErrorTypeInvalidRequest, Message: "bad request"},
			want: false,
		},
		{
			name: "404 not found",
			err:  &APIError{Status: 404, Type: ErrorTypeNotFound, Message: "not found"},
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := false
			if tt.err != nil {
				got = tt.err.IsRetryable()
			}
			if got != tt.want {
				t.Errorf("IsRetryable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAPIErrorIsFatal(t *testing.T) {
	tests := []struct {
		name string
		err  *APIError
		want bool
	}{
		{
			name: "400 is fatal",
			err:  &APIError{Status: 400, Type: ErrorTypeInvalidRequest, Message: "bad request"},
			want: true,
		},
		{
			name: "429 is not fatal",
			err:  &APIError{Status: 429, Type: ErrorTypeRateLimit, Message: "rate limited"},
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := false
			if tt.err != nil {
				got = tt.err.IsFatal()
			}
			if got != tt.want {
				t.Errorf("IsFatal() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAPIErrorRetryAfter(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   time.Duration
	}{
		{
			name:   "seconds format",
			header: "120",
			want:   120 * time.Second,
		},
		{
			name:   "RFC1123 date format",
			header: time.Now().Add(5 * time.Minute).Format(time.RFC1123),
			want:   5 * time.Minute,
		},
		{
			name:   "empty header",
			header: "",
			want:   0,
		},
		{
			name:   "invalid format",
			header: "not-a-number",
			want:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := http.Header{}
			if tt.header != "" {
				headers.Set("retry-after", tt.header)
			}
			err := &APIError{Status: 429, Headers: headers}
			got := err.RetryAfter()
			// Allow small delta for date parsing.
			if tt.want > 0 {
				if got <= 0 {
					t.Errorf("RetryAfter() = %v, want > 0", got)
				}
				if got > tt.want+2*time.Second || got < tt.want-2*time.Second {
					t.Errorf("RetryAfter() = %v, want ~%v", got, tt.want)
				}
			} else if got != 0 {
				t.Errorf("RetryAfter() = %v, want 0", got)
			}
		})
	}
}

func TestAPIErrorRateLimitReset(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   int64
	}{
		{
			name:   "valid reset timestamp",
			header: "1713800000",
			want:   1713800000,
		},
		{
			name:   "empty header",
			header: "",
			want:   0,
		},
		{
			name:   "invalid value",
			header: "not-a-number",
			want:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := http.Header{}
			if tt.header != "" {
				headers.Set("anthropic-ratelimit-unified-reset", tt.header)
			}
			err := &APIError{Status: 429, Headers: headers}
			got := err.RateLimitReset()
			if got != tt.want {
				t.Errorf("RateLimitReset() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAPIErrorRateLimitRemaining(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   int
	}{
		{
			name:   "valid remaining",
			header: "42",
			want:   42,
		},
		{
			name:   "empty header",
			header: "",
			want:   0,
		},
		{
			name:   "invalid value",
			header: "abc",
			want:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := http.Header{}
			if tt.header != "" {
				headers.Set("x-ratelimit-remaining", tt.header)
			}
			err := &APIError{Status: 429, Headers: headers}
			got := err.RateLimitRemaining()
			if got != tt.want {
				t.Errorf("RateLimitRemaining() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAPIErrorIsOverloaded(t *testing.T) {
	tests := []struct {
		name string
		err  *APIError
		want bool
	}{
		{
			name: "529 status",
			err:  &APIError{Status: 529, Message: "overloaded"},
			want: true,
		},
		{
			name: "overloaded_error type",
			err:  &APIError{Status: 500, Type: ErrorTypeOverloaded, Message: "overloaded"},
			want: true,
		},
		{
			name: "not overloaded",
			err:  &APIError{Status: 429, Type: ErrorTypeRateLimit, Message: "rate limited"},
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := false
			if tt.err != nil {
				got = tt.err.IsOverloaded()
			}
			if got != tt.want {
				t.Errorf("IsOverloaded() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAPIErrorIsRateLimit(t *testing.T) {
	tests := []struct {
		name string
		err  *APIError
		want bool
	}{
		{
			name: "429 status",
			err:  &APIError{Status: 429, Message: "rate limited"},
			want: true,
		},
		{
			name: "rate_limit_error type",
			err:  &APIError{Status: 400, Type: ErrorTypeRateLimit, Message: "rate limited"},
			want: true,
		},
		{
			name: "not rate limit",
			err:  &APIError{Status: 500, Type: ErrorTypeAPIError, Message: "internal error"},
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := false
			if tt.err != nil {
				got = tt.err.IsRateLimit()
			}
			if got != tt.want {
				t.Errorf("IsRateLimit() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAPIErrorIsPromptTooLong(t *testing.T) {
	tests := []struct {
		name string
		err  *APIError
		want bool
	}{
		{
			name: "prompt is too long",
			err:  &APIError{Status: 400, Message: "prompt is too long: 250000 tokens > 200000"},
			want: true,
		},
		{
			name: "context_length_exceeded",
			err:  &APIError{Status: 400, Message: "context_length_exceeded"},
			want: true,
		},
		{
			name: "model_context_window_exceeded",
			err:  &APIError{Status: 400, Message: "model_context_window_exceeded"},
			want: true,
		},
		{
			name: "context_window_exceeded",
			err:  &APIError{Status: 400, Message: "context_window_exceeded"},
			want: true,
		},
		{
			name: "not prompt too long",
			err:  &APIError{Status: 400, Message: "bad request"},
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := false
			if tt.err != nil {
				got = tt.err.IsPromptTooLong()
			}
			if got != tt.want {
				t.Errorf("IsPromptTooLong() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAPIErrorIsAuthError(t *testing.T) {
	tests := []struct {
		name string
		err  *APIError
		want bool
	}{
		{
			name: "401 status",
			err:  &APIError{Status: 401, Message: "unauthorized"},
			want: true,
		},
		{
			name: "403 status",
			err:  &APIError{Status: 403, Message: "forbidden"},
			want: true,
		},
		{
			name: "authentication_error type",
			err:  &APIError{Status: 400, Type: ErrorTypeAuthentication, Message: "auth failed"},
			want: true,
		},
		{
			name: "permission_error type",
			err:  &APIError{Status: 400, Type: ErrorTypePermission, Message: "no permission"},
			want: true,
		},
		{
			name: "not auth error",
			err:  &APIError{Status: 500, Message: "internal error"},
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := false
			if tt.err != nil {
				got = tt.err.IsAuthError()
			}
			if got != tt.want {
				t.Errorf("IsAuthError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseAPIError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       []byte
		wantStatus int
		wantType   APIErrorType
		wantMsg    string
	}{
		{
			name:       "JSON error body with type",
			statusCode: 429,
			body:       []byte(`{"error":{"type":"rate_limit_error","message":"Rate limit exceeded"}}`),
			wantStatus: 429,
			wantType:   ErrorTypeRateLimit,
			wantMsg:    "Rate limit exceeded",
		},
		{
			name:       "JSON error body overloaded",
			statusCode: 529,
			body:       []byte(`{"error":{"type":"overloaded_error","message":"Anthropic is overloaded"}}`),
			wantStatus: 529,
			wantType:   ErrorTypeOverloaded,
			wantMsg:    "Anthropic is overloaded",
		},
		{
			name:       "JSON error body authentication",
			statusCode: 401,
			body:       []byte(`{"error":{"type":"authentication_error","message":"Invalid API key"}}`),
			wantStatus: 401,
			wantType:   ErrorTypeAuthentication,
			wantMsg:    "Invalid API key",
		},
		{
			name:       "invalid JSON body falls back to status text",
			statusCode: 500,
			body:       []byte(`not json`),
			wantStatus: 500,
			wantType:   "",
			wantMsg:    "Internal Server Error",
		},
		{
			name:       "empty body falls back to status text",
			statusCode: 404,
			body:       []byte{},
			wantStatus: 404,
			wantType:   "",
			wantMsg:    "Not Found",
		},
		{
			name:       "nil body falls back to status text",
			statusCode: 503,
			body:       nil,
			wantStatus: 503,
			wantType:   "",
			wantMsg:    "Service Unavailable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{
				StatusCode: tt.statusCode,
				Header:     http.Header{},
			}
			got := ParseAPIError(resp, tt.body)
			if got == nil {
				t.Fatal("ParseAPIError() returned nil")
			}
			if got.Status != tt.wantStatus {
				t.Errorf("Status = %d, want %d", got.Status, tt.wantStatus)
			}
			if got.Type != tt.wantType {
				t.Errorf("Type = %q, want %q", got.Type, tt.wantType)
			}
			if got.Message != tt.wantMsg {
				t.Errorf("Message = %q, want %q", got.Message, tt.wantMsg)
			}
		})
	}
}

func TestParseAPIErrorWithHeaders(t *testing.T) {
	headers := http.Header{}
	headers.Set("request-id", "req-123")
	headers.Set("retry-after", "60")
	headers.Set("anthropic-ratelimit-unified-reset", "1713800000")

	resp := &http.Response{
		StatusCode: 429,
		Header:     headers,
	}
	body := []byte(`{"error":{"type":"rate_limit_error","message":"Rate limit exceeded"}}`)

	err := ParseAPIError(resp, body)
	if err == nil {
		t.Fatal("ParseAPIError() returned nil")
	}
	if err.RequestID != "req-123" {
		t.Errorf("RequestID = %q, want req-123", err.RequestID)
	}
	if err.RetryAfter() != 60*time.Second {
		t.Errorf("RetryAfter() = %v, want 60s", err.RetryAfter())
	}
	if err.RateLimitReset() != 1713800000 {
		t.Errorf("RateLimitReset() = %v, want 1713800000", err.RateLimitReset())
	}
}

func TestParseAPIErrorNilResponse(t *testing.T) {
	got := ParseAPIError(nil, nil)
	if got != nil {
		t.Errorf("ParseAPIError(nil, nil) = %v, want nil", got)
	}
}

func TestIsRetryableStatusCode(t *testing.T) {
	tests := []struct {
		statusCode int
		want       bool
	}{
		{408, true},
		{429, true},
		{500, true},
		{502, true},
		{503, true},
		{504, true},
		{529, true},
		{400, false},
		{401, false},
		{403, false},
		{404, false},
		{200, false},
		{201, false},
	}

	for _, tt := range tests {
		t.Run(http.StatusText(tt.statusCode), func(t *testing.T) {
			got := IsRetryableStatusCode(tt.statusCode)
			if got != tt.want {
				t.Errorf("IsRetryableStatusCode(%d) = %v, want %v", tt.statusCode, got, tt.want)
			}
		})
	}
}

func TestIsFatalStatusCode(t *testing.T) {
	tests := []struct {
		statusCode int
		want       bool
	}{
		{400, true},
		{401, true},
		{403, true},
		{404, true},
		{405, true},
		{422, true},
		{429, false},
		{500, false},
		{502, false},
		{503, false},
		{200, false},
	}

	for _, tt := range tests {
		t.Run(http.StatusText(tt.statusCode), func(t *testing.T) {
			got := IsFatalStatusCode(tt.statusCode)
			if got != tt.want {
				t.Errorf("IsFatalStatusCode(%d) = %v, want %v", tt.statusCode, got, tt.want)
			}
		})
	}
}

func TestAPIErrorErrorString(t *testing.T) {
	tests := []struct {
		name string
		err  *APIError
		want string
	}{
		{
			name: "with type",
			err:  &APIError{Status: 429, Type: ErrorTypeRateLimit, Message: "rate limited"},
			want: "anthropic api error: status=429 type=rate_limit_error message=rate limited",
		},
		{
			name: "without type",
			err:  &APIError{Status: 500, Message: "internal error"},
			want: "anthropic api error: status=500 message=internal error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsImageSizeError(t *testing.T) {
	tests := []struct {
		name string
		msg  string
		want bool
	}{
		{"image exceeds maximum", "image exceeds 5 MB maximum: 5316852 bytes > 5242880 bytes", true},
		{"no match", "some other error", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsImageSizeError(tt.msg); got != tt.want {
				t.Errorf("IsImageSizeError(%q) = %v, want %v", tt.msg, got, tt.want)
			}
		})
	}
}

func TestIsManyImageDimensionError(t *testing.T) {
	tests := []struct {
		name string
		msg  string
		want bool
	}{
		{"many-image dimension", "image dimensions exceed 2000px for many-image requests", true},
		{"no match", "some other error", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsManyImageDimensionError(tt.msg); got != tt.want {
				t.Errorf("IsManyImageDimensionError(%q) = %v, want %v", tt.msg, got, tt.want)
			}
		})
	}
}

func TestIsMediaSizeError(t *testing.T) {
	tests := []struct {
		name string
		msg  string
		want bool
	}{
		{"image size", "image exceeds 5 MB maximum", true},
		{"many-image", "image dimensions exceed 2000px for many-image", true},
		{"PDF pages", "maximum of 20 PDF pages exceeded", true},
		{"no match", "some other error", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsMediaSizeError(tt.msg); got != tt.want {
				t.Errorf("IsMediaSizeError(%q) = %v, want %v", tt.msg, got, tt.want)
			}
		})
	}
}

func TestCountMediaItems(t *testing.T) {
	tests := []struct {
		name string
		msgs []message.Message
		want int
	}{
		{
			name: "no media",
			msgs: []message.Message{
				{Content: []message.ContentPart{message.TextPart("hello")}},
			},
			want: 0,
		},
		{
			name: "one image",
			msgs: []message.Message{
				{Content: []message.ContentPart{message.ImagePart("image/jpeg", "abc")}},
			},
			want: 1,
		},
		{
			name: "mixed content",
			msgs: []message.Message{
				{Content: []message.ContentPart{
					message.TextPart("hello"),
					message.ImagePart("image/jpeg", "a"),
					message.ImagePart("image/png", "b"),
					{Type: "document"},
				}},
			},
			want: 3,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CountMediaItems(tt.msgs); got != tt.want {
				t.Errorf("CountMediaItems() = %d, want %d", got, tt.want)
			}
		})
	}
}
