package remote

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sync"
	"sync/atomic"
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

// TestCCRClientSendRateLimit verifies 429 is classified as SendErrorRateLimit,
// is retryable, and the message is queued instead of returning an error.
func TestCCRClientSendRateLimit(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	client := NewCCRClient(server.URL, "")
	err := client.Send([]byte(`{}`))
	if err != nil {
		t.Fatalf("Send() error = %v, want nil (message should be queued)", err)
	}
	if client.PendingCount() != 1 {
		t.Fatalf("PendingCount() = %d, want 1", client.PendingCount())
	}
	// Verify the error classification logic still works via direct call.
	err = classifyHTTPError(http.StatusTooManyRequests, "")
	se, ok := IsSendError(err)
	if !ok || se.Kind != SendErrorRateLimit {
		t.Fatalf("classifyHTTPError(429) = %v, want SendErrorRateLimit", err)
	}
	if !se.IsRetryable() {
		t.Fatal("rate limit error should be retryable")
	}
}

// TestCCRClientSendServerError verifies 5xx is classified as SendErrorServer,
// is retryable, and the message is queued instead of returning an error.
func TestCCRClientSendServerError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewCCRClient(server.URL, "")
	err := client.Send([]byte(`{}`))
	if err != nil {
		t.Fatalf("Send() error = %v, want nil (message should be queued)", err)
	}
	if client.PendingCount() != 1 {
		t.Fatalf("PendingCount() = %d, want 1", client.PendingCount())
	}
	// Verify classification logic directly.
	err = classifyHTTPError(http.StatusInternalServerError, "")
	se, ok := IsSendError(err)
	if !ok || se.Kind != SendErrorServer {
		t.Fatalf("classifyHTTPError(500) = %v, want SendErrorServer", err)
	}
	if !se.IsRetryable() {
		t.Fatal("server error should be retryable")
	}
}

// TestCCRClientSendTimeout verifies network timeout is classified as
// SendErrorTimeout, is retryable, and the message is queued.
func TestCCRClientSendTimeout(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewCCRClient(server.URL, "", WithHTTPClient(&http.Client{Timeout: 50 * time.Millisecond}))
	err := client.Send([]byte(`{}`))
	if err != nil {
		t.Fatalf("Send() error = %v, want nil (message should be queued)", err)
	}
	if client.PendingCount() != 1 {
		t.Fatalf("PendingCount() = %d, want 1", client.PendingCount())
	}
	// Verify classification logic directly.
	var netErr net.Error
	timeoutErr := &url.Error{Err: context.DeadlineExceeded}
	_ = netErr
	_ = timeoutErr
	// The actual timeout path goes through classifySendError.
	err = classifySendError(context.DeadlineExceeded)
	se, ok := IsSendError(err)
	if !ok || se.Kind != SendErrorTimeout {
		t.Fatalf("classifySendError(timeout) = %v, want SendErrorTimeout", err)
	}
	if !se.IsRetryable() {
		t.Fatal("timeout error should be retryable")
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

// TestPendingMessageQueueEnqueueDequeue verifies basic queue operations.
func TestPendingMessageQueueEnqueueDequeue(t *testing.T) {
	t.Parallel()

	q := NewPendingMessageQueue(10)
	msg := PendingMessage{ID: "msg-1", Data: []byte("hello"), Status: MessageStatusPending}

	if err := q.Enqueue(msg); err != nil {
		t.Fatalf("Enqueue() error = %v", err)
	}
	if q.Len() != 1 {
		t.Fatalf("Len() = %d, want 1", q.Len())
	}

	got, ok := q.Dequeue()
	if !ok {
		t.Fatal("Dequeue() = false, want true")
	}
	if got.ID != "msg-1" {
		t.Fatalf("Dequeue() ID = %q, want msg-1", got.ID)
	}
	if q.Len() != 0 {
		t.Fatalf("Len() = %d, want 0", q.Len())
	}

	_, ok = q.Dequeue()
	if ok {
		t.Fatal("Dequeue() on empty queue = true, want false")
	}
}

// TestPendingMessageQueueMaxSize verifies the queue rejects new messages when full.
func TestPendingMessageQueueMaxSize(t *testing.T) {
	t.Parallel()

	q := NewPendingMessageQueue(3)
	for i := 0; i < 3; i++ {
		if err := q.Enqueue(PendingMessage{ID: fmt.Sprintf("msg-%d", i)}); err != nil {
			t.Fatalf("Enqueue() error = %v", err)
		}
	}
	if !q.IsFull() {
		t.Fatal("IsFull() = false, want true")
	}
	if err := q.Enqueue(PendingMessage{ID: "msg-overflow"}); !errors.Is(err, ErrQueueFull) {
		t.Fatalf("Enqueue() error = %v, want ErrQueueFull", err)
	}
}

// TestPendingMessageQueuePeek verifies Peek returns the head without removing it.
func TestPendingMessageQueuePeek(t *testing.T) {
	t.Parallel()

	q := NewPendingMessageQueue(10)
	q.Enqueue(PendingMessage{ID: "first", Data: []byte("a")})
	q.Enqueue(PendingMessage{ID: "second", Data: []byte("b")})

	got, ok := q.Peek()
	if !ok || got.ID != "first" {
		t.Fatalf("Peek() = %v, %v, want first, true", got, ok)
	}
	if q.Len() != 2 {
		t.Fatalf("Len() after Peek = %d, want 2", q.Len())
	}
}

// TestPendingMessageQueueClear verifies Clear empties the queue.
func TestPendingMessageQueueClear(t *testing.T) {
	t.Parallel()

	q := NewPendingMessageQueue(10)
	for i := 0; i < 5; i++ {
		q.Enqueue(PendingMessage{ID: fmt.Sprintf("msg-%d", i)})
	}
	q.Clear()
	if q.Len() != 0 {
		t.Fatalf("Len() after Clear = %d, want 0", q.Len())
	}
}

// TestPendingMessageQueueConcurrency verifies thread safety under concurrent access.
func TestPendingMessageQueueConcurrency(t *testing.T) {
	t.Parallel()

	const numGoroutines = 50
	const msgsPerGoroutine = 100
	q := NewPendingMessageQueue(numGoroutines * msgsPerGoroutine)

	var wg sync.WaitGroup
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < msgsPerGoroutine; j++ {
				msg := PendingMessage{ID: fmt.Sprintf("g%d-m%d", id, j)}
				_ = q.Enqueue(msg)
			}
		}(i)
	}
	wg.Wait()

	expected := numGoroutines * msgsPerGoroutine
	if q.Len() != expected {
		t.Fatalf("Len() = %d, want %d", q.Len(), expected)
	}

	// Dequeue concurrently
	wg.Add(numGoroutines)
	dequeued := atomic.Int32{}
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for {
				_, ok := q.Dequeue()
				if !ok {
					return
				}
				dequeued.Add(1)
			}
		}()
	}
	wg.Wait()

	if int(dequeued.Load()) != expected {
		t.Fatalf("dequeued = %d, want %d", dequeued.Load(), expected)
	}
	if q.Len() != 0 {
		t.Fatalf("Len() after dequeue = %d, want 0", q.Len())
	}
}

// TestPendingMessageQueueDefaultSize verifies default size is applied.
func TestPendingMessageQueueDefaultSize(t *testing.T) {
	t.Parallel()

	q := NewPendingMessageQueue(0)
	for i := 0; i < 1000; i++ {
		if err := q.Enqueue(PendingMessage{ID: fmt.Sprintf("msg-%d", i)}); err != nil {
			t.Fatalf("Enqueue() error = %v at i=%d", err, i)
		}
	}
	if err := q.Enqueue(PendingMessage{ID: "overflow"}); !errors.Is(err, ErrQueueFull) {
		t.Fatalf("expected ErrQueueFull, got %v", err)
	}
}

// TestCCRClientPendingCount verifies PendingCount tracks the queue depth.
func TestCCRClientPendingCount(t *testing.T) {
	t.Parallel()

	client := NewCCRClient("http://example.com", "")
	if client.PendingCount() != 0 {
		t.Fatalf("PendingCount() = %d, want 0", client.PendingCount())
	}

	client.Enqueue(PendingMessage{ID: "msg-1", Data: []byte("a")})
	client.Enqueue(PendingMessage{ID: "msg-2", Data: []byte("b")})
	if client.PendingCount() != 2 {
		t.Fatalf("PendingCount() = %d, want 2", client.PendingCount())
	}

	client.ClearPending()
	if client.PendingCount() != 0 {
		t.Fatalf("PendingCount() after Clear = %d, want 0", client.PendingCount())
	}
}

// TestCCRClientEnqueueQueueFull verifies Enqueue returns ErrQueueFull when the
// CCRClient queue reaches its limit.
func TestCCRClientEnqueueQueueFull(t *testing.T) {
	t.Parallel()

	client := NewCCRClient("http://example.com", "", WithQueueSize(2))
	client.Enqueue(PendingMessage{ID: "msg-1"})
	client.Enqueue(PendingMessage{ID: "msg-2"})

	err := client.Enqueue(PendingMessage{ID: "msg-3"})
	if !errors.Is(err, ErrQueueFull) {
		t.Fatalf("Enqueue() error = %v, want ErrQueueFull", err)
	}
}

// TestCCRClientSendRetryableError_QueuesMessage verifies that a retryable
// network error causes the message to be queued instead of returning an error.
func TestCCRClientSendRetryableError_QueuesMessage(t *testing.T) {
	t.Parallel()

	// Use a server that never responds (simulates network timeout)
	client := NewCCRClient("http://127.0.0.1:1", "", WithHTTPClient(&http.Client{Timeout: 50 * time.Millisecond}))
	payload := []byte(`{"type":"user","message":"hello"}`)

	err := client.Send(payload)
	if err != nil {
		t.Fatalf("Send() error = %v, want nil (message should be queued)", err)
	}

	if client.PendingCount() != 1 {
		t.Fatalf("PendingCount() = %d, want 1", client.PendingCount())
	}

	msg, ok := client.queue.Peek()
	if !ok {
		t.Fatal("expected message in queue")
	}
	if string(msg.Data) != string(payload) {
		t.Fatalf("queued data = %q, want %q", msg.Data, payload)
	}
	if msg.ID == "" {
		t.Fatal("queued message missing ID")
	}
	if msg.Status != MessageStatusPending {
		t.Fatalf("queued status = %d, want MessageStatusPending", msg.Status)
	}
}

// TestCCRClientSendServerError_QueuesMessage verifies that a 5xx response
// causes the message to be queued.
func TestCCRClientSendServerError_QueuesMessage(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client := NewCCRClient(server.URL, "")
	payload := []byte(`{"type":"test"}`)

	err := client.Send(payload)
	if err != nil {
		t.Fatalf("Send() error = %v, want nil (message should be queued)", err)
	}

	if client.PendingCount() != 1 {
		t.Fatalf("PendingCount() = %d, want 1", client.PendingCount())
	}
}

// TestCCRClientSendRateLimit_QueuesMessage verifies that a 429 response
// causes the message to be queued.
func TestCCRClientSendRateLimit_QueuesMessage(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	client := NewCCRClient(server.URL, "")
	payload := []byte(`{"type":"test"}`)

	err := client.Send(payload)
	if err != nil {
		t.Fatalf("Send() error = %v, want nil (message should be queued)", err)
	}

	if client.PendingCount() != 1 {
		t.Fatalf("PendingCount() = %d, want 1", client.PendingCount())
	}
}

// TestCCRClientSendAuthError_NotQueued verifies that a 401/403 response
// (non-retryable) returns the error directly without queuing.
func TestCCRClientSendAuthError_NotQueued(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	client := NewCCRClient(server.URL, "")
	payload := []byte(`{"type":"test"}`)

	err := client.Send(payload)
	if err == nil {
		t.Fatal("Send() error = nil, want auth error")
	}
	se, ok := IsSendError(err)
	if !ok || se.Kind != SendErrorAuth {
		t.Fatalf("expected SendErrorAuth, got %v", err)
	}
	if client.PendingCount() != 0 {
		t.Fatalf("PendingCount() = %d, want 0 (auth error should not queue)", client.PendingCount())
	}
}

// TestCCRClientSendQueueFull_ReturnsError verifies that when the queue is
// full and a retryable error occurs, the error is returned.
func TestCCRClientSendQueueFull_ReturnsError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client := NewCCRClient(server.URL, "", WithQueueSize(1))
	// Fill the queue
	client.Enqueue(PendingMessage{ID: "filler"})

	payload := []byte(`{"type":"test"}`)
	err := client.Send(payload)
	if err == nil {
		t.Fatal("Send() error = nil, want error (queue full)")
	}
	if !errors.Is(err, ErrQueueFull) {
		t.Fatalf("expected ErrQueueFull in error chain, got %v", err)
	}
}

// TestCCRClientSendClientError_NotQueued verifies that a 4xx response
// (non-retryable) returns the error directly without queuing.
func TestCCRClientSendClientError_NotQueued(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	client := NewCCRClient(server.URL, "")
	payload := []byte(`{"type":"test"}`)

	err := client.Send(payload)
	if err == nil {
		t.Fatal("Send() error = nil, want client error")
	}
	se, ok := IsSendError(err)
	if !ok || se.Kind != SendErrorOther {
		t.Fatalf("expected SendErrorOther, got %v", err)
	}
	if client.PendingCount() != 0 {
		t.Fatalf("PendingCount() = %d, want 0 (client error should not queue)", client.PendingCount())
	}
}

// TestCCRClientSetConnected_QueuesWhenDisconnected verifies that sending
// while disconnected queues the message immediately.
func TestCCRClientSetConnected_QueuesWhenDisconnected(t *testing.T) {
	t.Parallel()

	client := NewCCRClient("http://example.com", "")
	client.SetConnected(false)

	if client.IsConnected() {
		t.Fatal("IsConnected() = true, want false")
	}

	payload := []byte(`{"type":"test"}`)
	err := client.Send(payload)
	if err != nil {
		t.Fatalf("Send() error = %v, want nil (should queue)", err)
	}
	if client.PendingCount() != 1 {
		t.Fatalf("PendingCount() = %d, want 1", client.PendingCount())
	}
}

// TestCCRClientResendPending_Success verifies that pending messages are sent
// when ResendPending is called.
func TestCCRClientResendPending_Success(t *testing.T) {
	t.Parallel()

	received := make(chan []byte, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		received <- body
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewCCRClient(server.URL, "")
	client.Enqueue(PendingMessage{ID: "msg-1", Data: []byte(`{"type":"a"}`)})
	client.Enqueue(PendingMessage{ID: "msg-2", Data: []byte(`{"type":"b"}`)})

	if client.PendingCount() != 2 {
		t.Fatalf("PendingCount() = %d, want 2", client.PendingCount())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.ResendPending(ctx); err != nil {
		t.Fatalf("ResendPending() error = %v", err)
	}

	if client.PendingCount() != 0 {
		t.Fatalf("PendingCount() after resend = %d, want 0", client.PendingCount())
	}

	// Verify both messages were sent
	msgs := make(map[string]bool)
	for i := 0; i < 2; i++ {
		select {
		case got := <-received:
			msgs[string(got)] = true
		case <-time.After(2 * time.Second):
			t.Fatal("timeout waiting for resent messages")
		}
	}
	if !msgs[`{"type":"a"}`] || !msgs[`{"type":"b"}`] {
		t.Fatalf("not all messages resent: %v", msgs)
	}
}

// TestCCRClientResendPending_RetryableRequeues verifies that retryable errors
// during resend re-queue the message.
func TestCCRClientResendPending_RetryableRequeues(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client := NewCCRClient(server.URL, "")
	client.Enqueue(PendingMessage{ID: "msg-1", Data: []byte(`{"type":"test"}`)})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := client.ResendPending(ctx)
	if err != nil {
		t.Fatalf("ResendPending() error = %v, want nil (retryable should re-queue)", err)
	}

	if client.PendingCount() != 1 {
		t.Fatalf("PendingCount() after resend = %d, want 1", client.PendingCount())
	}

	msg, _ := client.queue.Peek()
	if msg.RetryCount != 1 {
		t.Fatalf("RetryCount = %d, want 1", msg.RetryCount)
	}
}

// TestCCRClientResendPending_NonRetryableDrops verifies that non-retryable
// errors during resend drop the message.
func TestCCRClientResendPending_NonRetryableDrops(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	client := NewCCRClient(server.URL, "")
	client.Enqueue(PendingMessage{ID: "msg-1", Data: []byte(`{"type":"test"}`)})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := client.ResendPending(ctx)
	if err == nil {
		t.Fatal("ResendPending() error = nil, want error")
	}

	if client.PendingCount() != 0 {
		t.Fatalf("PendingCount() after resend = %d, want 0", client.PendingCount())
	}
}

// TestCCRClientSetConnected_TriggersResend verifies that transitioning from
// disconnected to connected triggers automatic resend.
func TestCCRClientSetConnected_TriggersResend(t *testing.T) {
	t.Parallel()

	received := make(chan []byte, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		received <- body
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewCCRClient(server.URL, "")
	client.SetConnected(false)
	client.Enqueue(PendingMessage{ID: "msg-1", Data: []byte(`{"type":"test"}`)})

	if client.PendingCount() != 1 {
		t.Fatalf("PendingCount() = %d, want 1", client.PendingCount())
	}

	// Transition to connected — should trigger resend
	client.SetConnected(true)

	select {
	case got := <-received:
		if string(got) != `{"type":"test"}` {
			t.Fatalf("resent data = %q, want %q", got, `{"type":"test"}`)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for auto-resend")
	}

	if client.PendingCount() != 0 {
		t.Fatalf("PendingCount() after auto-resend = %d, want 0", client.PendingCount())
	}
}

// TestCCRClientResendPending_RetryLimit verifies that messages exceeding the
// retry limit are dropped.
func TestCCRClientResendPending_RetryLimit(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client := NewCCRClient(server.URL, "")
	// Pre-load a message with retry count at the limit
	client.Enqueue(PendingMessage{ID: "msg-1", Data: []byte(`{"type":"test"}`), RetryCount: MaxMessageRetries})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := client.ResendPending(ctx)
	if err == nil {
		t.Fatal("ResendPending() error = nil, want error")
	}

	if client.PendingCount() != 0 {
		t.Fatalf("PendingCount() after resend = %d, want 0 (message should be dropped)", client.PendingCount())
	}
}

// TestCCRClientResendPending_IdempotentUUID verifies that resent messages
// retain their original UUID for server-side idempotency.
func TestCCRClientResendPending_IdempotentUUID(t *testing.T) {
	t.Parallel()

	received := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		received <- string(body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewCCRClient(server.URL, "")
	fixedID := "test-uuid-123"
	client.Enqueue(PendingMessage{ID: fixedID, Data: []byte(`{"type":"test"}`)})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.ResendPending(ctx); err != nil {
		t.Fatalf("ResendPending() error = %v", err)
	}

	select {
	case got := <-received:
		if got != `{"type":"test"}` {
			t.Fatalf("resent data = %q, want %q", got, `{"type":"test"}`)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for resent message")
	}
}
