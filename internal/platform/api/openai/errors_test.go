package openai

import (
	"net/http"
	"testing"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/model"
)

// TestParseAPIErrorWithJSONBody verifies structured error parsing from an OpenAI-style JSON error body.
func TestParseAPIErrorWithJSONBody(t *testing.T) {
	body := []byte(`{"error":{"message":"Rate limit exceeded","type":"rate_limit_error","param":null,"code":"rate_limit_exceeded"}}`)
	resp := &http.Response{
		StatusCode: 429,
		Header:     http.Header{"Retry-After": []string{"5"}},
	}

	err := ParseAPIError(resp, body)
	if err == nil {
		t.Fatal("ParseAPIError() = nil, want non-nil")
	}
	if err.Status != 429 {
		t.Fatalf("status = %d, want 429", err.Status)
	}
	if err.Type != ErrorTypeRateLimit {
		t.Fatalf("type = %q, want rate_limit_error", err.Type)
	}
	if err.Message != "Rate limit exceeded" {
		t.Fatalf("message = %q, want Rate limit exceeded", err.Message)
	}
	if err.Code != "rate_limit_exceeded" {
		t.Fatalf("code = %q, want rate_limit_exceeded", err.Code)
	}
}

// TestParseAPIErrorWithEmptyBody verifies fallback to status text when body is empty.
func TestParseAPIErrorWithEmptyBody(t *testing.T) {
	resp := &http.Response{StatusCode: 500}

	err := ParseAPIError(resp, nil)
	if err == nil {
		t.Fatal("ParseAPIError() = nil, want non-nil")
	}
	if err.Status != 500 {
		t.Fatalf("status = %d, want 500", err.Status)
	}
	if err.Message != "Internal Server Error" {
		t.Fatalf("message = %q, want Internal Server Error", err.Message)
	}
}

// TestParseAPIErrorWithInvalidJSON verifies fallback when JSON is malformed.
func TestParseAPIErrorWithInvalidJSON(t *testing.T) {
	resp := &http.Response{StatusCode: 400}
	body := []byte(`not json`)

	err := ParseAPIError(resp, body)
	if err == nil {
		t.Fatal("ParseAPIError() = nil, want non-nil")
	}
	if err.Status != 400 {
		t.Fatalf("status = %d, want 400", err.Status)
	}
	if err.Message != "Bad Request" {
		t.Fatalf("message = %q, want Bad Request", err.Message)
	}
}

// TestAPIErrorIsRetryable verifies the retry classification matrix.
func TestAPIErrorIsRetryable(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		errType   APIErrorType
		retryable bool
	}{
		{"rate limit 429", 429, ErrorTypeRateLimit, true},
		{"rate limit no type", 429, "", true},
		{"timeout 408", 408, "", true},
		{"auth 401", 401, ErrorTypeAuthentication, true},
		{"server error 500", 500, ErrorTypeServerError, true},
		{"bad gateway 502", 502, "", true},
		{"service unavailable 503", 503, "", true},
		{"gateway timeout 504", 504, "", true},
		{"bad request 400", 400, ErrorTypeInvalidRequest, false},
		{"forbidden 403", 403, "", false},
		{"not found 404", 404, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &APIError{Status: tt.status, Type: tt.errType}
			got := err.IsRetryable()
			if got != tt.retryable {
				t.Errorf("IsRetryable() = %v, want %v", got, tt.retryable)
			}
		})
	}
}

// TestAPIErrorRetryAfter verifies retry-after header parsing.
func TestAPIErrorRetryAfter(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   time.Duration
	}{
		{"seconds", "5", 5 * time.Second},
		{"zero seconds", "0", 0},
		{"rfc1123 date", time.Now().Add(10 * time.Second).Format(time.RFC1123), 10 * time.Second},
		{"empty", "", 0},
		{"invalid", "abc", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &APIError{
				Status:  429,
				Headers: http.Header{"Retry-After": []string{tt.header}},
			}
			got := err.RetryAfter()
			// Allow 1s tolerance for RFC1123 date parsing.
			if got < tt.want-time.Second || got > tt.want+time.Second {
				t.Errorf("RetryAfter() = %v, want ~%v", got, tt.want)
			}
		})
	}
}

// TestAPIErrorIsRateLimit verifies rate limit detection.
func TestAPIErrorIsRateLimit(t *testing.T) {
	if !(&APIError{Status: 429}).IsRateLimit() {
		t.Error("IsRateLimit(429) = false, want true")
	}
	if !(&APIError{Status: 400, Type: ErrorTypeRateLimit}).IsRateLimit() {
		t.Error("IsRateLimit(type=rate_limit_error) = false, want true")
	}
	if (&APIError{Status: 400}).IsRateLimit() {
		t.Error("IsRateLimit(400) = true, want false")
	}
	if (&APIError{}).IsRateLimit() {
		t.Error("IsRateLimit(nil-ish) = true, want false")
	}
}

// TestAPIErrorIsPromptTooLong verifies prompt-too-long detection for OpenAI errors.
func TestAPIErrorIsPromptTooLong(t *testing.T) {
	tests := []struct {
		msg  string
		want bool
	}{
		{"This model's maximum context length is 8192 tokens", true},
		{"context_length_exceeded", true},
		{"Context_Length_Exceeded", true},
		{"context length exceeded", true},
		{"Rate limit exceeded", false},
		{"Invalid API key", false},
	}
	for _, tt := range tests {
		t.Run(tt.msg, func(t *testing.T) {
			err := &APIError{Status: 400, Message: tt.msg}
			if got := err.IsPromptTooLong(); got != tt.want {
				t.Errorf("IsPromptTooLong() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestAPIErrorIsAuthError verifies authentication error detection.
func TestAPIErrorIsAuthError(t *testing.T) {
	if !(&APIError{Status: 401}).IsAuthError() {
		t.Error("IsAuthError(401) = false, want true")
	}
	if !(&APIError{Status: 403}).IsAuthError() {
		t.Error("IsAuthError(403) = false, want true")
	}
	if !(&APIError{Status: 400, Type: ErrorTypeAuthentication}).IsAuthError() {
		t.Error("IsAuthError(type=authentication_error) = false, want true")
	}
	if (&APIError{Status: 429}).IsAuthError() {
		t.Error("IsAuthError(429) = true, want false")
	}
}

// TestAPIErrorRateLimitRemaining verifies remaining requests header parsing.
func TestAPIErrorRateLimitRemaining(t *testing.T) {
	err := &APIError{
		Status:  429,
		Headers: http.Header{"X-Ratelimit-Remaining-Requests": []string{"42"}},
	}
	if got := err.RateLimitRemaining(); got != 42 {
		t.Fatalf("RateLimitRemaining() = %d, want 42", got)
	}
}

// TestAPIErrorRateLimitReset verifies reset timestamp header parsing.
func TestAPIErrorRateLimitReset(t *testing.T) {
	resetTime := time.Now().Add(5 * time.Minute).UTC().Truncate(time.Second)
	err := &APIError{
		Status:  429,
		Headers: http.Header{"X-Ratelimit-Reset-Requests": []string{resetTime.Format(time.RFC3339)}},
	}
	got := err.RateLimitReset()
	if !got.Equal(resetTime) {
		t.Fatalf("RateLimitReset() = %v, want %v", got, resetTime)
	}
}

// TestAPIErrorImplementsRetryableError verifies OpenAI APIError satisfies model.RetryableError.
func TestAPIErrorImplementsRetryableError(t *testing.T) {
	var _ model.RetryableError = (&APIError{})

	err := &APIError{Status: 429, Type: ErrorTypeRateLimit, Message: "rate limit"}
	if !err.IsRetryable() {
		t.Error("IsRetryable() = false, want true for rate limit")
	}
	if err.RetryAfter() != 0 {
		t.Fatalf("RetryAfter() = %v, want 0 (no header)", err.RetryAfter())
	}
}

// TestAPIErrorErrorString verifies the Error() string format.
func TestAPIErrorErrorString(t *testing.T) {
	err := &APIError{Status: 429, Type: ErrorTypeRateLimit, Message: "too many requests"}
	want := "openai api error: status=429 type=rate_limit_error message=too many requests"
	if got := err.Error(); got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}

	errNoType := &APIError{Status: 500, Message: "server error"}
	wantNoType := "openai api error: status=500 message=server error"
	if got := errNoType.Error(); got != wantNoType {
		t.Fatalf("Error() = %q, want %q", got, wantNoType)
	}
}

// TestIsRetryableStatusCode verifies the status-code-based helper.
func TestIsRetryableStatusCode(t *testing.T) {
	tests := []struct {
		code int
		want bool
	}{
		{408, true}, {429, true}, {502, true}, {503, true}, {504, true},
		{500, true}, {501, true}, {505, true},
		{400, false}, {401, false}, {403, false}, {404, false}, {422, false},
	}
	for _, tt := range tests {
		if got := IsRetryableStatusCode(tt.code); got != tt.want {
			t.Errorf("IsRetryableStatusCode(%d) = %v, want %v", tt.code, got, tt.want)
		}
	}
}

// TestIsFatalStatusCode verifies the fatal status-code helper.
func TestIsFatalStatusCode(t *testing.T) {
	tests := []struct {
		code int
		want bool
	}{
		{400, true}, {401, true}, {403, true}, {404, true}, {405, true}, {422, true},
		{408, false}, {429, false}, {500, false}, {502, false}, {503, false},
	}
	for _, tt := range tests {
		if got := IsFatalStatusCode(tt.code); got != tt.want {
			t.Errorf("IsFatalStatusCode(%d) = %v, want %v", tt.code, got, tt.want)
		}
	}
}
