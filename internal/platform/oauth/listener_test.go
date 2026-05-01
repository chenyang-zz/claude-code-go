package oauth

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

// callbackClient does not follow redirects, so tests can inspect the 302
// emitted by the listener instead of chasing it to a remote URL.
var callbackClient = &http.Client{
	Timeout: 5 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

func TestAuthCodeListener_StartAndPort(t *testing.T) {
	listener := NewAuthCodeListener("")
	defer listener.Close()

	port, err := listener.Start(0)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if port <= 0 {
		t.Fatalf("expected a positive port, got %d", port)
	}
	if listener.Port() != port {
		t.Fatalf("Port() = %d, want %d", listener.Port(), port)
	}
	if listener.CallbackPath() != DefaultCallbackPath {
		t.Fatalf("CallbackPath() = %q, want %q", listener.CallbackPath(), DefaultCallbackPath)
	}
}

func TestAuthCodeListener_StartTwiceFails(t *testing.T) {
	listener := NewAuthCodeListener("")
	defer listener.Close()

	if _, err := listener.Start(0); err != nil {
		t.Fatalf("first Start: %v", err)
	}
	if _, err := listener.Start(0); err == nil {
		t.Fatalf("second Start should fail")
	}
}

func TestAuthCodeListener_AutomaticCallback_Success(t *testing.T) {
	listener := NewAuthCodeListener("")
	defer listener.Close()

	port, err := listener.Start(0)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	const wantState = "expected-state-abc"
	const wantCode = "auth-code-xyz"

	type waitResult struct {
		code string
		err  error
	}
	waitCh := make(chan waitResult, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		code, err := listener.WaitForAuthorization(ctx, wantState, nil)
		waitCh <- waitResult{code: code, err: err}
	}()

	time.Sleep(20 * time.Millisecond) // let WaitForAuthorization arm

	respCh := launchCallback(t, port, "/callback", url.Values{
		"code":  {wantCode},
		"state": {wantState},
	})

	r := <-waitCh
	if r.err != nil {
		t.Fatalf("WaitForAuthorization returned error: %v", r.err)
	}
	if r.code != wantCode {
		t.Fatalf("WaitForAuthorization returned code %q, want %q", r.code, wantCode)
	}
	if !listener.HasPendingResponse() {
		t.Fatalf("HasPendingResponse() should be true after capture but before redirect")
	}

	listener.HandleSuccessRedirect([]string{ScopeUserInference}, SuccessRedirectURLs{})

	if listener.HasPendingResponse() {
		t.Fatalf("HasPendingResponse() should be false after redirect")
	}

	resp := awaitResponse(t, respCh)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusFound)
	}
	if resp.Header.Get("Location") != DefaultClaudeAISuccessURL {
		t.Fatalf("Location = %q, want %q", resp.Header.Get("Location"), DefaultClaudeAISuccessURL)
	}
}

func TestAuthCodeListener_SuccessRedirect_ConsoleScope(t *testing.T) {
	listener := NewAuthCodeListener("")
	defer listener.Close()

	port, err := listener.Start(0)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	const wantState = "expected-state"
	const wantCode = "code-1"

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_, _ = listener.WaitForAuthorization(ctx, wantState, nil)
	}()
	time.Sleep(20 * time.Millisecond)

	respCh := launchCallback(t, port, "/callback", url.Values{
		"code":  {wantCode},
		"state": {wantState},
	})

	// Wait until the callback is parked.
	deadline := time.Now().Add(2 * time.Second)
	for !listener.HasPendingResponse() && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if !listener.HasPendingResponse() {
		t.Fatalf("HasPendingResponse() never became true")
	}

	listener.HandleSuccessRedirect([]string{ScopeOrgCreateAPIKey, ScopeUserProfile}, SuccessRedirectURLs{})

	resp := awaitResponse(t, respCh)
	defer resp.Body.Close()
	if resp.Header.Get("Location") != DefaultConsoleSuccessURL {
		t.Fatalf("Location = %q, want console success URL", resp.Header.Get("Location"))
	}
}

func TestAuthCodeListener_StateMismatch(t *testing.T) {
	listener := NewAuthCodeListener("")
	defer listener.Close()

	port, err := listener.Start(0)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	type waitResult struct {
		code string
		err  error
	}
	waitCh := make(chan waitResult, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		code, err := listener.WaitForAuthorization(ctx, "expected-state", nil)
		waitCh <- waitResult{code: code, err: err}
	}()
	time.Sleep(20 * time.Millisecond)

	resp := getCallback(t, port, "/callback", url.Values{
		"code":  {"code-1"},
		"state": {"WRONG"},
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	r := <-waitCh
	if r.code != "" {
		t.Fatalf("expected empty code on state mismatch, got %q", r.code)
	}
	if r.err == nil || !strings.Contains(r.err.Error(), "Invalid state") {
		t.Fatalf("expected state-mismatch error, got %v", r.err)
	}
	if listener.HasPendingResponse() {
		t.Fatalf("HasPendingResponse() should be false after state-mismatch failure")
	}
}

func TestAuthCodeListener_MissingCode(t *testing.T) {
	listener := NewAuthCodeListener("")
	defer listener.Close()

	port, err := listener.Start(0)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	resCh := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_, err := listener.WaitForAuthorization(ctx, "expected-state", nil)
		resCh <- err
	}()
	time.Sleep(20 * time.Millisecond)

	resp := getCallback(t, port, "/callback", url.Values{"state": {"expected-state"}})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	err = <-resCh
	if err == nil || !strings.Contains(err.Error(), "Authorization code not found") {
		t.Fatalf("expected missing-code error, got %v", err)
	}
}

func TestAuthCodeListener_UnknownPathReturns404(t *testing.T) {
	listener := NewAuthCodeListener("")
	defer listener.Close()

	port, err := listener.Start(0)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	resp := getCallback(t, port, "/somewhere-else", nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestAuthCodeListener_CloseReleasesPendingResponse(t *testing.T) {
	listener := NewAuthCodeListener("")
	port, err := listener.Start(0)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_, _ = listener.WaitForAuthorization(ctx, "expected-state", nil)
	}()
	time.Sleep(20 * time.Millisecond)

	respCh := launchCallback(t, port, "/callback", url.Values{
		"code":  {"code-1"},
		"state": {"expected-state"},
	})

	// Wait until the callback is parked.
	deadline := time.Now().Add(2 * time.Second)
	for !listener.HasPendingResponse() && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if !listener.HasPendingResponse() {
		t.Fatalf("HasPendingResponse() never became true before Close")
	}

	if err := listener.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	resp := awaitResponse(t, respCh)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusFound)
	}
}

func TestAuthCodeListener_ContextCancel(t *testing.T) {
	listener := NewAuthCodeListener("")
	defer listener.Close()

	if _, err := listener.Start(0); err != nil {
		t.Fatalf("Start: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	resCh := make(chan error, 1)
	go func() {
		_, err := listener.WaitForAuthorization(ctx, "expected-state", nil)
		resCh <- err
	}()
	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-resCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("WaitForAuthorization did not return after cancel")
	}
}

func TestAuthCodeListener_OnReady(t *testing.T) {
	listener := NewAuthCodeListener("")
	defer listener.Close()

	port, err := listener.Start(0)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	readyCh := make(chan struct{}, 1)
	resCh := make(chan struct{}, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_, _ = listener.WaitForAuthorization(ctx, "state", func() error {
			readyCh <- struct{}{}
			return nil
		})
		resCh <- struct{}{}
	}()

	select {
	case <-readyCh:
	case <-time.After(2 * time.Second):
		t.Fatalf("onReady was not invoked")
	}

	respCh := launchCallback(t, port, "/callback", url.Values{
		"code":  {"c"},
		"state": {"state"},
	})
	deadline := time.Now().Add(2 * time.Second)
	for !listener.HasPendingResponse() && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	listener.HandleSuccessRedirect([]string{ScopeUserInference}, SuccessRedirectURLs{})
	resp := awaitResponse(t, respCh)
	defer resp.Body.Close()
	<-resCh
}

func TestAuthCodeListener_WaitForAuthorizationTwiceFails(t *testing.T) {
	listener := NewAuthCodeListener("")
	defer listener.Close()

	if _, err := listener.Start(0); err != nil {
		t.Fatalf("Start: %v", err)
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()
		_, _ = listener.WaitForAuthorization(ctx, "state", nil)
	}()
	time.Sleep(20 * time.Millisecond)

	_, err := listener.WaitForAuthorization(context.Background(), "state", nil)
	if err == nil || !strings.Contains(err.Error(), "already in progress") {
		t.Fatalf("expected already-in-progress error, got %v", err)
	}

	time.Sleep(250 * time.Millisecond) // let the first wait return
}

func TestSuccessRedirectURLs_OverrideTakesPrecedence(t *testing.T) {
	urls := SuccessRedirectURLs{
		ClaudeAI: "https://override.example.com/claudeai",
		Console:  "https://override.example.com/console",
	}
	if got := urls.resolve([]string{ScopeUserInference}); got != urls.ClaudeAI {
		t.Fatalf("resolve(claudeai scope) = %q, want %q", got, urls.ClaudeAI)
	}
	if got := urls.resolve([]string{ScopeOrgCreateAPIKey}); got != urls.Console {
		t.Fatalf("resolve(console scope) = %q, want %q", got, urls.Console)
	}
}

// --- helpers ---

// getCallback issues a synchronous GET. Use only for paths that complete
// without parking the handler (state mismatch, missing code, 404).
func getCallback(t *testing.T, port int, path string, query url.Values) *http.Response {
	t.Helper()
	target := fmt.Sprintf("http://127.0.0.1:%d%s", port, path)
	if query != nil {
		target = target + "?" + query.Encode()
	}
	req, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := callbackClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", target, err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp
}

// launchCallback issues an asynchronous GET. Use for happy-path callbacks
// that park the handler waiting for HandleSuccessRedirect / Close.
func launchCallback(t *testing.T, port int, path string, query url.Values) <-chan callbackOutcome {
	t.Helper()
	respCh := make(chan callbackOutcome, 1)
	target := fmt.Sprintf("http://127.0.0.1:%d%s", port, path)
	if query != nil {
		target = target + "?" + query.Encode()
	}
	go func() {
		req, err := http.NewRequest(http.MethodGet, target, nil)
		if err != nil {
			respCh <- callbackOutcome{err: err}
			return
		}
		resp, err := callbackClient.Do(req)
		respCh <- callbackOutcome{resp: resp, err: err}
	}()
	return respCh
}

type callbackOutcome struct {
	resp *http.Response
	err  error
}

// awaitResponse blocks for the launched callback's response or fails the test.
func awaitResponse(t *testing.T, ch <-chan callbackOutcome) *http.Response {
	t.Helper()
	select {
	case out := <-ch:
		if out.err != nil {
			t.Fatalf("callback returned error: %v", out.err)
		}
		return out.resp
	case <-time.After(3 * time.Second):
		t.Fatalf("callback response did not arrive")
		return nil
	}
}
