package oauth

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// fakeUpstream stages an OAuth provider that returns canned token + profile
// responses. It is used by service tests to simulate the upstream without
// hitting api.anthropic.com.
type fakeUpstream struct {
	tokenStatus     int
	tokenBody       string
	profileStatus   int
	profileBody     string
	tokenCallCount  int32
	profileCallCount int32
}

func newFakeUpstream() *fakeUpstream {
	return &fakeUpstream{
		tokenStatus: http.StatusOK,
		tokenBody: `{
			"access_token": "fake-at",
			"refresh_token": "fake-rt",
			"expires_in": 3600,
			"scope": "user:profile user:inference",
			"account": {"uuid": "acct-1", "email_address": "u@example.com"},
			"organization": {"uuid": "org-1"}
		}`,
		profileStatus: http.StatusOK,
		profileBody: `{
			"account": {"uuid": "acct-1", "email": "u@example.com", "display_name": "U", "created_at": "2024-01-01T00:00:00Z"},
			"organization": {"uuid": "org-1", "organization_type": "claude_max", "rate_limit_tier": "default_claude_max_5x"}
		}`,
	}
}

func (f *fakeUpstream) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			atomic.AddInt32(&f.tokenCallCount, 1)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(f.tokenStatus)
			_, _ = w.Write([]byte(f.tokenBody))
		case "/profile":
			atomic.AddInt32(&f.profileCallCount, 1)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(f.profileStatus)
			_, _ = w.Write([]byte(f.profileBody))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

func TestOAuthService_AutomaticFlow_HappyPath(t *testing.T) {
	upstream := newFakeUpstream()
	srv := httptest.NewServer(upstream.handler())
	defer srv.Close()

	svc := NewOAuthService()

	type result struct {
		tokens *OAuthTokens
		err    error
	}
	resCh := make(chan result, 1)

	manualURLCh := make(chan string, 1)
	autoURLCh := make(chan string, 1)
	authURLHandler := func(manualURL, autoURL string) error {
		manualURLCh <- manualURL
		autoURLCh <- autoURL
		return nil
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		tokens, err := svc.StartOAuthFlow(ctx, StartOAuthFlowOptions{
			SkipBrowserOpen: true,
			TokenURL:        srv.URL + "/token",
			ProfileURL:      srv.URL + "/profile",
		}, authURLHandler)
		resCh <- result{tokens: tokens, err: err}
	}()

	autoURL := <-autoURLCh
	<-manualURLCh

	parsed, err := url.Parse(autoURL)
	if err != nil {
		t.Fatalf("parse auto URL: %v", err)
	}
	port := parsed.Query().Get("redirect_uri")
	if !strings.HasPrefix(port, "http://localhost:") {
		t.Fatalf("redirect_uri = %q, want http://localhost:* prefix", port)
	}

	state := parsed.Query().Get("state")
	if state == "" {
		t.Fatalf("state missing from auto URL")
	}

	// Hit the listener like a browser would.
	callbackResp, err := postCallback(t, port, state, "auth-code-1")
	if err != nil {
		t.Fatalf("callback: %v", err)
	}
	defer callbackResp.Body.Close()

	r := <-resCh
	if r.err != nil {
		t.Fatalf("StartOAuthFlow: %v", r.err)
	}
	if r.tokens == nil {
		t.Fatalf("nil tokens")
	}
	if r.tokens.AccessToken != "fake-at" || r.tokens.RefreshToken != "fake-rt" {
		t.Fatalf("token mismatch: %+v", r.tokens)
	}
	if r.tokens.SubscriptionType != SubscriptionMax {
		t.Fatalf("subscription = %q", r.tokens.SubscriptionType)
	}
	if r.tokens.TokenAccount == nil || r.tokens.TokenAccount.UUID != "acct-1" {
		t.Fatalf("TokenAccount = %+v", r.tokens.TokenAccount)
	}
	if r.tokens.TokenAccount.OrganizationUUID != "org-1" {
		t.Fatalf("OrganizationUUID = %q", r.tokens.TokenAccount.OrganizationUUID)
	}
	if atomic.LoadInt32(&upstream.tokenCallCount) != 1 {
		t.Fatalf("token endpoint hit %d times", upstream.tokenCallCount)
	}
	if atomic.LoadInt32(&upstream.profileCallCount) != 1 {
		t.Fatalf("profile endpoint hit %d times", upstream.profileCallCount)
	}
	if r.tokens.ExpiresAt == 0 {
		t.Fatalf("ExpiresAt should be non-zero")
	}
}

func TestOAuthService_ManualFlow_HappyPath(t *testing.T) {
	upstream := newFakeUpstream()
	srv := httptest.NewServer(upstream.handler())
	defer srv.Close()

	svc := NewOAuthService()
	resCh := make(chan struct {
		tokens *OAuthTokens
		err    error
	}, 1)

	authURLHandler := func(manualURL, autoURL string) error {
		// Once URLs have been built, simulate the user pasting a code.
		go func() {
			time.Sleep(20 * time.Millisecond)
			parsed, _ := url.Parse(manualURL)
			state := parsed.Query().Get("state")
			_ = svc.HandleManualAuthCodeInput(state, "manual-code")
		}()
		return nil
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		tokens, err := svc.StartOAuthFlow(ctx, StartOAuthFlowOptions{
			SkipBrowserOpen: true,
			TokenURL:        srv.URL + "/token",
			ProfileURL:      srv.URL + "/profile",
		}, authURLHandler)
		resCh <- struct {
			tokens *OAuthTokens
			err    error
		}{tokens, err}
	}()

	r := <-resCh
	if r.err != nil {
		t.Fatalf("StartOAuthFlow: %v", r.err)
	}
	if r.tokens.AccessToken != "fake-at" {
		t.Fatalf("token mismatch")
	}
	// Manual flow should not redirect, so HasPendingResponse() should be false
	// when we exit.
}

func TestOAuthService_RequiresAuthURLHandler(t *testing.T) {
	svc := NewOAuthService()
	_, err := svc.StartOAuthFlow(context.Background(), StartOAuthFlowOptions{}, nil)
	if err == nil || !strings.Contains(err.Error(), "authURLHandler is required") {
		t.Fatalf("expected authURLHandler error, got %v", err)
	}
}

func TestOAuthService_HandleManualAuthCodeInput_NoFlow(t *testing.T) {
	svc := NewOAuthService()
	if err := svc.HandleManualAuthCodeInput("state", "code"); err == nil {
		t.Fatalf("expected no-flow error")
	}
}

func TestOAuthService_HandleManualAuthCodeInput_StateMismatch(t *testing.T) {
	svc := NewOAuthService()
	resCh := make(chan error, 1)
	authURLHandler := func(manualURL, autoURL string) error {
		go func() {
			time.Sleep(20 * time.Millisecond)
			_ = svc.HandleManualAuthCodeInput("WRONG", "code")
		}()
		return nil
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_, err := svc.StartOAuthFlow(ctx, StartOAuthFlowOptions{SkipBrowserOpen: true}, authURLHandler)
		resCh <- err
	}()

	err := <-resCh
	if err == nil || !strings.Contains(err.Error(), "manual state mismatch") {
		t.Fatalf("expected manual state-mismatch error, got %v", err)
	}
}

func TestOAuthService_TokenExchangeError_TriggersErrorRedirect(t *testing.T) {
	upstream := newFakeUpstream()
	upstream.tokenStatus = http.StatusUnauthorized
	upstream.tokenBody = `{"error":"invalid_grant"}`
	srv := httptest.NewServer(upstream.handler())
	defer srv.Close()

	svc := NewOAuthService()
	resCh := make(chan error, 1)
	manualURLCh := make(chan string, 1)
	autoURLCh := make(chan string, 1)
	authURLHandler := func(manualURL, autoURL string) error {
		manualURLCh <- manualURL
		autoURLCh <- autoURL
		return nil
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_, err := svc.StartOAuthFlow(ctx, StartOAuthFlowOptions{
			SkipBrowserOpen: true,
			TokenURL:        srv.URL + "/token",
			ProfileURL:      srv.URL + "/profile",
		}, authURLHandler)
		resCh <- err
	}()

	autoURL := <-autoURLCh
	<-manualURLCh

	parsed, _ := url.Parse(autoURL)
	state := parsed.Query().Get("state")
	redirectURI := parsed.Query().Get("redirect_uri")
	resp, err := postCallback(t, redirectURI, state, "code-1")
	if err != nil {
		t.Fatalf("callback: %v", err)
	}
	defer resp.Body.Close()

	err = <-resCh
	if err == nil || !strings.Contains(err.Error(), "invalid authorization code") {
		t.Fatalf("expected invalid-authorization-code error, got %v", err)
	}
}

func TestOAuthService_ProfileFetchFailureIsNonFatal(t *testing.T) {
	upstream := newFakeUpstream()
	upstream.profileStatus = http.StatusInternalServerError
	upstream.profileBody = `{"error":"oops"}`
	srv := httptest.NewServer(upstream.handler())
	defer srv.Close()

	svc := NewOAuthService()
	resCh := make(chan struct {
		tokens *OAuthTokens
		err    error
	}, 1)
	manualURLCh := make(chan string, 1)
	autoURLCh := make(chan string, 1)
	authURLHandler := func(manualURL, autoURL string) error {
		manualURLCh <- manualURL
		autoURLCh <- autoURL
		return nil
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		tokens, err := svc.StartOAuthFlow(ctx, StartOAuthFlowOptions{
			SkipBrowserOpen: true,
			TokenURL:        srv.URL + "/token",
			ProfileURL:      srv.URL + "/profile",
		}, authURLHandler)
		resCh <- struct {
			tokens *OAuthTokens
			err    error
		}{tokens, err}
	}()

	autoURL := <-autoURLCh
	<-manualURLCh
	parsed, _ := url.Parse(autoURL)
	state := parsed.Query().Get("state")
	redirectURI := parsed.Query().Get("redirect_uri")
	resp, err := postCallback(t, redirectURI, state, "code-1")
	if err != nil {
		t.Fatalf("callback: %v", err)
	}
	defer resp.Body.Close()

	r := <-resCh
	if r.err != nil {
		t.Fatalf("expected nil error when only profile fetch fails, got %v", r.err)
	}
	if r.tokens.AccessToken != "fake-at" {
		t.Fatalf("expected tokens despite profile failure")
	}
	if r.tokens.SubscriptionType != "" {
		t.Fatalf("SubscriptionType should be empty when profile fails, got %q", r.tokens.SubscriptionType)
	}
}

func TestOAuthService_ContextCancel(t *testing.T) {
	svc := NewOAuthService()
	ctx, cancel := context.WithCancel(context.Background())
	resCh := make(chan error, 1)
	go func() {
		_, err := svc.StartOAuthFlow(ctx, StartOAuthFlowOptions{SkipBrowserOpen: true}, func(_, _ string) error {
			return nil
		})
		resCh <- err
	}()
	time.Sleep(20 * time.Millisecond)
	cancel()
	select {
	case err := <-resCh:
		if err == nil {
			t.Fatalf("expected error after cancel, got nil")
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("StartOAuthFlow did not return after cancel")
	}
}

func TestOAuthService_Cleanup_NoFlow(t *testing.T) {
	svc := NewOAuthService()
	if err := svc.Cleanup(); err != nil {
		t.Fatalf("Cleanup with no flow should be a no-op, got %v", err)
	}
}

// --- helpers ---

// postCallback sends an HTTP GET to the localhost callback path. redirectURI
// is the redirect_uri query value from the auto URL (so it carries the
// listener's port). Returns the response asynchronously through a goroutine,
// since the listener parks the response until HandleSuccessRedirect releases
// it. We discard the eventual response body.
func postCallback(t *testing.T, redirectURI, state, code string) (*http.Response, error) {
	t.Helper()
	parsed, err := url.Parse(redirectURI)
	if err != nil {
		return nil, err
	}
	target := fmt.Sprintf("%s://%s%s?code=%s&state=%s", parsed.Scheme, parsed.Host, parsed.Path, url.QueryEscape(code), url.QueryEscape(state))

	respCh := make(chan *http.Response, 1)
	errCh := make(chan error, 1)
	go func() {
		client := &http.Client{
			Timeout: 5 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
		req, _ := http.NewRequest(http.MethodGet, target, nil)
		resp, err := client.Do(req)
		if err != nil {
			errCh <- err
			return
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		respCh <- resp
	}()
	select {
	case resp := <-respCh:
		return resp, nil
	case err := <-errCh:
		return nil, err
	case <-time.After(3 * time.Second):
		return nil, fmt.Errorf("callback request timed out")
	}
}
