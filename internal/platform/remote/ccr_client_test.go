package remote

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
	"github.com/sheepzhao/claude-code-go/pkg/sdk"
)

// TestCCRClientSendSuccess verifies Send delivers data and accepts 2xx.
func TestCCRClientSendSuccess(t *testing.T) {
	t.Parallel()

	received := make(chan []byte, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %q, want POST", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Fatalf("Content-Type = %q, want application/json", ct)
		}
		body := make([]byte, r.ContentLength)
		r.Body.Read(body)
		received <- body
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	client := NewCCRClient(server.URL, "session_123")
	payload := []byte(`{"type":"user","message":"hello"}`)
	if err := client.Send(payload); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	select {
	case got := <-received:
		if string(got) != string(payload) {
			t.Fatalf("server received = %q, want %q", got, payload)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for server to receive message")
	}
}

// TestCCRClientSendWithHeader verifies custom headers are forwarded.
func TestCCRClientSendWithHeader(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auth := r.Header.Get("Authorization"); auth != "Bearer token_xyz" {
			t.Fatalf("Authorization = %q, want Bearer token_xyz", auth)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewCCRClient(server.URL, "", WithHeader("Authorization", "Bearer token_xyz"))
	if err := client.Send([]byte(`{}`)); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
}

// TestCCRClientSendUserMessage verifies serialization and session ID injection.
func TestCCRClientSendUserMessage(t *testing.T) {
	t.Parallel()

	received := make(chan sdk.User, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var msg sdk.User
		if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		received <- msg
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewCCRClient(server.URL, "sess_42")
	msg := sdk.User{
		Base:    sdk.Base{Type: "user"},
		Message: "hello remote",
	}
	if err := client.SendUserMessage(context.Background(), msg); err != nil {
		t.Fatalf("SendUserMessage() error = %v", err)
	}

	select {
	case got := <-received:
		if got.Type != "user" {
			t.Fatalf("msg.Type = %q, want user", got.Type)
		}
		if got.Message != "hello remote" {
			t.Fatalf("msg.Message = %v, want hello remote", got.Message)
		}
		if got.SessionID != "sess_42" {
			t.Fatalf("msg.SessionID = %q, want sess_42", got.SessionID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for server to receive message")
	}
}

// TestCCRClientSendUserMessagePreservesExistingSessionID verifies existing session_id is not overwritten.
func TestCCRClientSendUserMessagePreservesExistingSessionID(t *testing.T) {
	t.Parallel()

	received := make(chan sdk.User, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var msg sdk.User
		json.NewDecoder(r.Body).Decode(&msg)
		received <- msg
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewCCRClient(server.URL, "sess_42")
	msg := sdk.User{
		Base:    sdk.Base{Type: "user", SessionID: "sess_99"},
		Message: "hello",
	}
	if err := client.SendUserMessage(context.Background(), msg); err != nil {
		t.Fatalf("SendUserMessage() error = %v", err)
	}

	select {
	case got := <-received:
		if got.SessionID != "sess_99" {
			t.Fatalf("msg.SessionID = %q, want sess_99", got.SessionID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}
}

// TestCCRClientSendAuthError verifies 401 is classified as SendErrorAuth.
func TestCCRClientSendAuthError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("unauthorized"))
	}))
	defer server.Close()

	client := NewCCRClient(server.URL, "")
	err := client.Send([]byte(`{}`))
	if err == nil {
		t.Fatal("Send() error = nil, want error")
	}
	se, ok := IsSendError(err)
	if !ok {
		t.Fatalf("error is not *SendError: %T", err)
	}
	if se.Kind != SendErrorAuth {
		t.Fatalf("SendError.Kind = %v, want SendErrorAuth", se.Kind)
	}
	if se.Status != http.StatusUnauthorized {
		t.Fatalf("SendError.Status = %d, want %d", se.Status, http.StatusUnauthorized)
	}
	if se.IsRetryable() {
		t.Fatal("auth error should not be retryable")
	}
}

// TestCCRClientSendForbiddenError verifies 403 is classified as SendErrorAuth.
func TestCCRClientSendForbiddenError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	client := NewCCRClient(server.URL, "")
	err := client.Send([]byte(`{}`))
	se, ok := IsSendError(err)
	if !ok {
		t.Fatalf("error is not *SendError: %T", err)
	}
	if se.Kind != SendErrorAuth {
		t.Fatalf("SendError.Kind = %v, want SendErrorAuth", se.Kind)
	}
}

// TestCCRClientSendRateLimit verifies 429 is classified as SendErrorRateLimit and is retryable.
func TestCCRClientSendRateLimit(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	client := NewCCRClient(server.URL, "")
	err := client.Send([]byte(`{}`))
	se, ok := IsSendError(err)
	if !ok {
		t.Fatalf("error is not *SendError: %T", err)
	}
	if se.Kind != SendErrorRateLimit {
		t.Fatalf("SendError.Kind = %v, want SendErrorRateLimit", se.Kind)
	}
	if !se.IsRetryable() {
		t.Fatal("rate limit error should be retryable")
	}
}

// TestCCRClientSendServerError verifies 5xx is classified as SendErrorServer and is retryable.
func TestCCRClientSendServerError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewCCRClient(server.URL, "")
	err := client.Send([]byte(`{}`))
	se, ok := IsSendError(err)
	if !ok {
		t.Fatalf("error is not *SendError: %T", err)
	}
	if se.Kind != SendErrorServer {
		t.Fatalf("SendError.Kind = %v, want SendErrorServer", se.Kind)
	}
	if !se.IsRetryable() {
		t.Fatal("server error should be retryable")
	}
}

// TestCCRClientSendTimeout verifies network timeout is classified as SendErrorTimeout.
func TestCCRClientSendTimeout(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewCCRClient(server.URL, "", WithHTTPClient(&http.Client{Timeout: 50 * time.Millisecond}))
	err := client.Send([]byte(`{}`))
	if err == nil {
		t.Fatal("Send() error = nil, want timeout error")
	}
	se, ok := IsSendError(err)
	if !ok {
		t.Fatalf("error is not *SendError: %T", err)
	}
	if se.Kind != SendErrorTimeout {
		t.Fatalf("SendError.Kind = %v, want SendErrorTimeout", se.Kind)
	}
}

// TestCCRClientNilEndpoint verifies empty endpoint returns an error.
func TestCCRClientNilEndpoint(t *testing.T) {
	t.Parallel()

	client := NewCCRClient("", "")
	err := client.Send([]byte(`{}`))
	if err == nil {
		t.Fatal("Send() error = nil, want error")
	}
	if !errors.Is(err, ErrStreamClosed) {
		// Any error is fine; just ensure it fails fast.
		if _, ok := IsSendError(err); ok {
			// classified error is also acceptable
			return
		}
	}
}

// TestCCRClientSendControlResponse verifies control response serialization.
func TestCCRClientSendControlResponse(t *testing.T) {
	t.Parallel()

	received := make(chan sdk.ControlResponse, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var resp sdk.ControlResponse
		json.NewDecoder(r.Body).Decode(&resp)
		received <- resp
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewCCRClient(server.URL, "")
	resp := sdk.ControlResponse{
		Type: "control_response",
		Response: sdk.ControlResponseInner{
			Subtype:   "success",
			RequestID: "req_1",
			Response:  map[string]any{"approved": true},
		},
	}
	if err := client.SendControlResponse(context.Background(), resp); err != nil {
		t.Fatalf("SendControlResponse() error = %v", err)
	}

	select {
	case got := <-received:
		if got.Response.RequestID != "req_1" {
			t.Fatalf("RequestID = %q, want req_1", got.Response.RequestID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}
}

// TestSendErrorString verifies SendError.Error() formatting.
func TestSendErrorString(t *testing.T) {
	t.Parallel()

	e := &SendError{Kind: SendErrorAuth, Message: "bad token", Status: 401}
	want := "ccr send error [auth]: bad token"
	if got := e.Error(); got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}
}

// TestSendErrorKindString verifies all kinds produce non-empty labels.
func TestSendErrorKindString(t *testing.T) {
	t.Parallel()

	cases := []struct {
		kind SendErrorKind
		want string
	}{
		{SendErrorNetwork, "network"},
		{SendErrorAuth, "auth"},
		{SendErrorTimeout, "timeout"},
		{SendErrorRateLimit, "rate_limit"},
		{SendErrorServer, "server"},
		{SendErrorOther, "other"},
		{SendErrorKind(999), "other"},
	}
	for _, tc := range cases {
		if got := tc.kind.String(); got != tc.want {
			t.Fatalf("Kind(%d).String() = %q, want %q", tc.kind, got, tc.want)
		}
	}
}

// TestDeriveEndpointFromEnv verifies explicit env-var endpoint.
func TestDeriveEndpointFromEnv(t *testing.T) {
	t.Setenv("CLAUDE_CODE_REMOTE_POST_URL", "https://api.example.com/messages")
	defer os.Unsetenv("CLAUDE_CODE_REMOTE_POST_URL")

	got := DeriveEndpoint(coreconfig.RemoteSessionConfig{StreamURL: "wss://other.com/stream"})
	if got != "https://api.example.com/messages" {
		t.Fatalf("DeriveEndpoint() = %q, want https://api.example.com/messages", got)
	}
}

// TestDeriveEndpointFromStreamURL verifies ws-to-http scheme replacement.
func TestDeriveEndpointFromStreamURL(t *testing.T) {
	os.Unsetenv("CLAUDE_CODE_REMOTE_POST_URL")

	cases := []struct {
		streamURL string
		want      string
	}{
		{"wss://claude.ai/sessions/123/stream", "https://claude.ai/sessions/123/stream"},
		{"ws://localhost:8080/stream", "http://localhost:8080/stream"},
	}
	for _, tc := range cases {
		got := DeriveEndpoint(coreconfig.RemoteSessionConfig{StreamURL: tc.streamURL})
		if got != tc.want {
			t.Fatalf("DeriveEndpoint(%q) = %q, want %q", tc.streamURL, got, tc.want)
		}
	}
}

// TestDeriveEndpointEmpty verifies empty result when no source is available.
func TestDeriveEndpointEmpty(t *testing.T) {
	os.Unsetenv("CLAUDE_CODE_REMOTE_POST_URL")

	got := DeriveEndpoint(coreconfig.RemoteSessionConfig{})
	if got != "" {
		t.Fatalf("DeriveEndpoint() = %q, want empty string", got)
	}
}

// TestCCRClientSend_401RetrySuccess verifies that a 401 response triggers token
// refresh and the request is retried with the new token.
func TestCCRClientSend_401RetrySuccess(t *testing.T) {
	t.Parallel()

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		auth := r.Header.Get("Authorization")
		if requestCount == 1 {
			// First request with old token should get 401
			if auth != "Bearer old-token" {
				t.Fatalf("request 1 auth = %q, want Bearer old-token", auth)
			}
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		// Second request with new token should succeed
		if auth != "Bearer new-token" {
			t.Fatalf("request 2 auth = %q, want Bearer new-token", auth)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	provider := &mockTokenProvider{token: "old-token"}
	provider.refreshFunc = func() (string, error) {
		provider.token = "new-token"
		return "new-token", nil
	}

	client := NewCCRClient(server.URL, "", WithTokenProvider(provider))
	err := client.Send([]byte(`{}`))
	if err != nil {
		t.Fatalf("Send() error = %v, want nil after retry", err)
	}
	if requestCount != 2 {
		t.Fatalf("server received %d requests, want 2", requestCount)
	}
}

// TestCCRClientSend_401NoTokenProvider verifies 401 is returned directly when
// no token provider is configured.
func TestCCRClientSend_401NoTokenProvider(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	client := NewCCRClient(server.URL, "")
	err := client.Send([]byte(`{}`))
	if err == nil {
		t.Fatal("Send() error = nil, want 401 error")
	}
	se, ok := IsSendError(err)
	if !ok || se.Kind != SendErrorAuth {
		t.Fatalf("expected SendErrorAuth, got %v", err)
	}
}

// TestCCRClientSend_401SameToken verifies that when refresh returns the same
// token, the original 401 is returned without infinite retry loops.
func TestCCRClientSend_401SameToken(t *testing.T) {
	t.Parallel()

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	provider := &mockTokenProvider{
		token: "same-token",
		refreshFunc: func() (string, error) {
			return "same-token", nil
		},
	}

	client := NewCCRClient(server.URL, "", WithTokenProvider(provider))
	err := client.Send([]byte(`{}`))
	if err == nil {
		t.Fatal("Send() error = nil, want 401 error")
	}
	if requestCount != 1 {
		t.Fatalf("server received %d requests, want 1 (no retry)", requestCount)
	}
}

// TestCCRClientSend_401RefreshFails verifies that when refresh fails, the
// original 401 is returned.
func TestCCRClientSend_401RefreshFails(t *testing.T) {
	t.Parallel()

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	provider := &mockTokenProvider{
		token: "some-token",
		refreshFunc: func() (string, error) {
			return "", errors.New("refresh failed")
		},
	}

	client := NewCCRClient(server.URL, "", WithTokenProvider(provider))
	err := client.Send([]byte(`{}`))
	if err == nil {
		t.Fatal("Send() error = nil, want 401 error")
	}
	if requestCount != 1 {
		t.Fatalf("server received %d requests, want 1", requestCount)
	}
}

// TestCCRClientReadInternalEvents_401Retry verifies 401 recovery on GET requests.
func TestCCRClientReadInternalEvents_401Retry(t *testing.T) {
	t.Parallel()

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		auth := r.Header.Get("Authorization")
		if requestCount == 1 {
			if auth != "Bearer old-token" {
				t.Fatalf("request 1 auth = %q, want Bearer old-token", auth)
			}
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if auth != "Bearer new-token" {
			t.Fatalf("request 2 auth = %q, want Bearer new-token", auth)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data":[],"next_cursor":""}`))
	}))
	defer server.Close()

	provider := &mockTokenProvider{token: "old-token"}
	provider.refreshFunc = func() (string, error) {
		provider.token = "new-token"
		return "new-token", nil
	}

	client := NewCCRClient(server.URL, "", WithTokenProvider(provider))
	events, err := client.ReadInternalEvents(context.Background())
	if err != nil {
		t.Fatalf("ReadInternalEvents() error = %v, want nil after retry", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected 0 events, got %d", len(events))
	}
	if requestCount != 2 {
		t.Fatalf("server received %d requests, want 2", requestCount)
	}
}

// TestCCRClientAuthState verifies AuthState delegates to the token provider.
func TestCCRClientAuthState(t *testing.T) {
	t.Parallel()

	provider := NewEnvTokenProvider()
	client := NewCCRClient("http://example.com", "", WithTokenProvider(provider))

	state := client.AuthState()
	// Should not panic and should return a valid AuthState
	if state.RefreshCount != 0 {
		t.Fatalf("expected refresh count 0, got %d", state.RefreshCount)
	}
}

// TestCCRClientAuthState_NilProvider verifies AuthState returns zero value when
// no token provider is configured.
func TestCCRClientAuthState_NilProvider(t *testing.T) {
	t.Parallel()

	client := NewCCRClient("http://example.com", "")
	state := client.AuthState()
	if state.Token != "" {
		t.Fatalf("expected empty token, got %q", state.Token)
	}
}

// mockTokenProvider is a test double for TokenProvider.
type mockTokenProvider struct {
	token       string
	refreshFunc func() (string, error)
}

func (m *mockTokenProvider) Token() string {
	return m.token
}

func (m *mockTokenProvider) Refresh() (string, error) {
	if m.refreshFunc != nil {
		return m.refreshFunc()
	}
	return m.token, nil
}
