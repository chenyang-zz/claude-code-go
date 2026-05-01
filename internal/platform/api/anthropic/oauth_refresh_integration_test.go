package anthropic

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
	"github.com/sheepzhao/claude-code-go/internal/core/model"
	"github.com/sheepzhao/claude-code-go/internal/platform/oauth"
)

// oauthRefreshFixture stages a HTTP server that simultaneously hosts the
// Anthropic /v1/messages endpoint, the OAuth token endpoint, and the OAuth
// profile endpoint so end-to-end auto-refresh flows can be exercised against
// a single httptest.Server without external dependencies.
type oauthRefreshFixture struct {
	server *httptest.Server

	// firstAccessToken is the token persisted in the credential store
	// before the test starts. The /v1/messages handler uses it to decide
	// whether to return 401 or 200 on each request.
	firstAccessToken string

	// rotatedAccessToken is the access token returned by /token. After a
	// successful refresh this is what the retried request must present.
	rotatedAccessToken string

	// messagesStatusForFirstToken governs how /v1/messages reacts when it
	// observes firstAccessToken. Defaults to 401.
	messagesStatusForFirstToken int

	// tokenStatus governs how /token reacts. Defaults to 200.
	tokenStatus int
	// tokenBody overrides the default refresh response body when non-empty.
	tokenBody string

	// Counters for assertions.
	messagesCalls int32
	tokenCalls    int32
	profileCalls  int32

	// authzHeaders captures the Authorization header observed on each
	// /v1/messages call, in arrival order.
	mu           chan struct{}
	authzHeaders []string
}

func newOAuthRefreshFixture(t *testing.T) *oauthRefreshFixture {
	t.Helper()
	f := &oauthRefreshFixture{
		firstAccessToken:            "stale-at",
		rotatedAccessToken:          "rotated-at",
		messagesStatusForFirstToken: http.StatusUnauthorized,
		tokenStatus:                 http.StatusOK,
		mu:                          make(chan struct{}, 1),
	}

	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/messages":
			atomic.AddInt32(&f.messagesCalls, 1)
			authz := r.Header.Get("authorization")
			f.mu <- struct{}{}
			f.authzHeaders = append(f.authzHeaders, authz)
			<-f.mu

			expectedFirst := "Bearer " + f.firstAccessToken
			expectedRotated := "Bearer " + f.rotatedAccessToken

			switch authz {
			case expectedFirst:
				w.WriteHeader(f.messagesStatusForFirstToken)
				_, _ = w.Write([]byte(`{"error":{"message":"OAuth token has expired"}}`))
			case expectedRotated:
				w.Header().Set("content-type", "text/event-stream")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("event: content_block_delta\n"))
				_, _ = w.Write([]byte("data: {\"delta\":{\"type\":\"text_delta\",\"text\":\"ok\"}}\n\n"))
			default:
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":{"message":"unexpected token"}}`))
			}
		case "/token":
			atomic.AddInt32(&f.tokenCalls, 1)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(f.tokenStatus)
			body := f.tokenBody
			if body == "" {
				body = `{
					"access_token": "` + f.rotatedAccessToken + `",
					"refresh_token": "rotated-rt",
					"expires_in": 3600,
					"scope": "user:profile user:inference",
					"account": {"uuid": "acct-1", "email_address": "u@example.com"},
					"organization": {"uuid": "org-1"}
				}`
			}
			_, _ = w.Write([]byte(body))
		case "/profile":
			atomic.AddInt32(&f.profileCalls, 1)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"account": {"uuid": "acct-1", "email": "u@example.com", "display_name": "U", "created_at": "2024-01-01T00:00:00Z"},
				"organization": {"uuid": "org-1", "organization_type": "claude_max", "rate_limit_tier": "default_claude_max_5x"}
			}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(f.server.Close)
	return f
}

func (f *oauthRefreshFixture) anthropicURL() string { return f.server.URL }
func (f *oauthRefreshFixture) tokenURL() string     { return f.server.URL + "/token" }
func (f *oauthRefreshFixture) profileURL() string   { return f.server.URL + "/profile" }

// makeRefresher constructs a TokenRefresher backed by a fresh credential
// store seeded with the provided token bundle. The bundle's ExpiresAt
// controls whether MaybeRefresh treats it as expired.
func makeRefresher(t *testing.T, fix *oauthRefreshFixture, tokens *oauth.OAuthTokens) (*oauth.TokenRefresher, *oauth.OAuthCredentialStore) {
	t.Helper()
	store, err := oauth.NewOAuthCredentialStore(t.TempDir())
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	if tokens != nil {
		decision, err := store.Save(tokens)
		if err != nil {
			t.Fatalf("save seed tokens: %v", err)
		}
		if !decision.Persisted {
			t.Fatalf("seed tokens not persisted: %s", decision.SkipReason)
		}
	}
	refresher, err := oauth.NewTokenRefresher(oauth.TokenRefresherConfig{
		Service: oauth.NewOAuthService(),
		Store:   store,
		Options: oauth.RefreshTokenOptions{
			TokenURL:   fix.tokenURL(),
			ProfileURL: fix.profileURL(),
		},
	})
	if err != nil {
		t.Fatalf("create refresher: %v", err)
	}
	return refresher, store
}

// minimalRequest returns a model.Request payload sufficient for the SSE
// pipeline to produce one event without requiring a full conversation.
func minimalRequest() model.Request {
	return model.Request{
		Model: "claude-sonnet-4-5",
		Messages: []message.Message{
			{
				Role: message.RoleUser,
				Content: []message.ContentPart{
					{Type: "text", Text: "hello"},
				},
			},
		},
	}
}

// drainStream pulls all events from the model stream until it closes,
// returning the first event observed.
func drainStream(stream <-chan model.Event) []model.Event {
	var events []model.Event
	for evt := range stream {
		events = append(events, evt)
	}
	return events
}

func TestClientOAuthAutoRefresh_Reactive401Retry(t *testing.T) {
	fix := newOAuthRefreshFixture(t)
	// Seed fresh tokens so MaybeRefresh skips the preemptive path; the 401
	// response forces the reactive path.
	refresher, store := makeRefresher(t, fix, &oauth.OAuthTokens{
		AccessToken:  fix.firstAccessToken,
		RefreshToken: "stale-rt",
		ExpiresAt:    time.Now().Add(1 * time.Hour).UnixMilli(),
		Scopes:       []string{"user:inference", "user:profile"},
	})

	client := NewClient(Config{
		AuthToken:      fix.firstAccessToken,
		BaseURL:        fix.anthropicURL(),
		HTTPClient:     fix.server.Client(),
		IsFirstParty:   true,
		TokenRefresher: refresher,
	})

	stream, err := client.Stream(context.Background(), minimalRequest())
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}
	events := drainStream(stream)
	if len(events) == 0 {
		t.Fatal("expected at least one event after retry")
	}
	if events[0].Type != model.EventTypeTextDelta || events[0].Text != "ok" {
		t.Fatalf("first event = %+v, want text delta 'ok'", events[0])
	}
	if got := atomic.LoadInt32(&fix.messagesCalls); got != 2 {
		t.Fatalf("expected /v1/messages called 2 times (1 401 + 1 retry), got %d", got)
	}
	if got := atomic.LoadInt32(&fix.tokenCalls); got != 1 {
		t.Fatalf("expected /token called 1 time, got %d", got)
	}
	if len(fix.authzHeaders) != 2 {
		t.Fatalf("expected 2 captured headers, got %d", len(fix.authzHeaders))
	}
	if fix.authzHeaders[0] != "Bearer "+fix.firstAccessToken {
		t.Fatalf("first attempt header = %q, want stale token", fix.authzHeaders[0])
	}
	if fix.authzHeaders[1] != "Bearer "+fix.rotatedAccessToken {
		t.Fatalf("retry attempt header = %q, want rotated token", fix.authzHeaders[1])
	}
	// Verify the rotated tokens were persisted.
	stored, err := store.Load()
	if err != nil {
		t.Fatalf("store.Load: %v", err)
	}
	if stored == nil || stored.AccessToken != fix.rotatedAccessToken {
		t.Fatalf("expected store to contain rotated token, got %+v", stored)
	}
}

func TestClientOAuthAutoRefresh_PreemptiveBeforeExpiry(t *testing.T) {
	fix := newOAuthRefreshFixture(t)
	// Seed expired tokens so MaybeRefresh refreshes before the request leaves.
	refresher, _ := makeRefresher(t, fix, &oauth.OAuthTokens{
		AccessToken:  fix.firstAccessToken,
		RefreshToken: "stale-rt",
		ExpiresAt:    time.Now().Add(-1 * time.Minute).UnixMilli(),
		Scopes:       []string{"user:inference", "user:profile"},
	})

	client := NewClient(Config{
		AuthToken:      fix.firstAccessToken,
		BaseURL:        fix.anthropicURL(),
		HTTPClient:     fix.server.Client(),
		IsFirstParty:   true,
		TokenRefresher: refresher,
	})

	stream, err := client.Stream(context.Background(), minimalRequest())
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}
	events := drainStream(stream)
	if len(events) == 0 {
		t.Fatal("expected at least one event")
	}
	if events[0].Type != model.EventTypeTextDelta || events[0].Text != "ok" {
		t.Fatalf("first event = %+v, want text delta 'ok'", events[0])
	}
	// Preemptive path means /v1/messages should be hit only once with the
	// rotated token, never with the stale one.
	if got := atomic.LoadInt32(&fix.messagesCalls); got != 1 {
		t.Fatalf("expected /v1/messages called once, got %d", got)
	}
	if got := atomic.LoadInt32(&fix.tokenCalls); got != 1 {
		t.Fatalf("expected /token called once, got %d", got)
	}
	if len(fix.authzHeaders) != 1 {
		t.Fatalf("expected 1 captured header, got %d", len(fix.authzHeaders))
	}
	if fix.authzHeaders[0] != "Bearer "+fix.rotatedAccessToken {
		t.Fatalf("preemptive request header = %q, want rotated token", fix.authzHeaders[0])
	}
}

func TestClientOAuthAutoRefresh_RefreshFailureReturnsErrOAuthRefreshFailed(t *testing.T) {
	fix := newOAuthRefreshFixture(t)
	fix.tokenStatus = http.StatusInternalServerError
	fix.tokenBody = `{"error":"upstream boom"}`

	refresher, store := makeRefresher(t, fix, &oauth.OAuthTokens{
		AccessToken:  fix.firstAccessToken,
		RefreshToken: "stale-rt",
		ExpiresAt:    time.Now().Add(1 * time.Hour).UnixMilli(),
		Scopes:       []string{"user:inference", "user:profile"},
	})

	client := NewClient(Config{
		AuthToken:      fix.firstAccessToken,
		BaseURL:        fix.anthropicURL(),
		HTTPClient:     fix.server.Client(),
		IsFirstParty:   true,
		TokenRefresher: refresher,
	})

	_, err := client.Stream(context.Background(), minimalRequest())
	if err == nil {
		t.Fatal("expected error when refresh upstream returns 500")
	}
	if !errors.Is(err, oauth.ErrOAuthRefreshFailed) {
		t.Fatalf("expected ErrOAuthRefreshFailed in chain, got %v", err)
	}
	// Ensure credentials are NOT deleted on refresh failure.
	stored, err := store.Load()
	if err != nil {
		t.Fatalf("store.Load: %v", err)
	}
	if stored == nil || stored.AccessToken != fix.firstAccessToken {
		t.Fatalf("expected credentials retained after refresh failure, got %+v", stored)
	}
}

func TestClientOAuthAutoRefresh_NoRefresherSkipsHooks(t *testing.T) {
	fix := newOAuthRefreshFixture(t)
	// Make /v1/messages return 200 SSE on the first call so we can verify the
	// client behaves like batch-218/219 when no refresher is supplied.
	fix.messagesStatusForFirstToken = http.StatusOK

	client := NewClient(Config{
		AuthToken:    fix.firstAccessToken,
		BaseURL:      fix.anthropicURL(),
		HTTPClient:   fix.server.Client(),
		IsFirstParty: true,
		// TokenRefresher: nil — the auto-refresh path must stay disabled.
	})

	// Override the /v1/messages handler so the 200 path actually emits SSE.
	customServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&fix.messagesCalls, 1)
		fix.authzHeaders = append(fix.authzHeaders, r.Header.Get("authorization"))
		w.Header().Set("content-type", "text/event-stream")
		_, _ = w.Write([]byte("event: content_block_delta\n"))
		_, _ = w.Write([]byte("data: {\"delta\":{\"type\":\"text_delta\",\"text\":\"plain\"}}\n\n"))
	}))
	t.Cleanup(customServer.Close)

	client = NewClient(Config{
		AuthToken:    fix.firstAccessToken,
		BaseURL:      customServer.URL,
		HTTPClient:   customServer.Client(),
		IsFirstParty: true,
	})

	stream, err := client.Stream(context.Background(), minimalRequest())
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}
	events := drainStream(stream)
	if len(events) == 0 || events[0].Type != model.EventTypeTextDelta || events[0].Text != "plain" {
		t.Fatalf("expected 'plain' text delta, got %+v", events)
	}
	if got := atomic.LoadInt32(&fix.tokenCalls); got != 0 {
		t.Fatalf("expected /token NOT called when refresher is nil, got %d", got)
	}
}

// TestClientOAuthAutoRefresh_RetryFailsTwiceReturnsAPIError ensures the
// reactive 401 path only retries once: when the second attempt also fails,
// we surface the standard mapped APIError rather than looping forever.
func TestClientOAuthAutoRefresh_RetryFailsTwiceReturnsAPIError(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/messages":
			atomic.AddInt32(&calls, 1)
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":{"message":"OAuth token has expired"}}`))
		case "/token":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token":  "rotated-at",
				"refresh_token": "rotated-rt",
				"expires_in":    3600,
				"scope":         "user:inference user:profile",
				"account":       map[string]string{"uuid": "acct-1", "email_address": "u@example.com"},
				"organization":  map[string]string{"uuid": "org-1"},
			})
		case "/profile":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"account":{},"organization":{}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	store, err := oauth.NewOAuthCredentialStore(t.TempDir())
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	if _, err := store.Save(&oauth.OAuthTokens{
		AccessToken:  "stale-at",
		RefreshToken: "stale-rt",
		ExpiresAt:    time.Now().Add(1 * time.Hour).UnixMilli(),
		Scopes:       []string{"user:inference", "user:profile"},
	}); err != nil {
		t.Fatalf("seed tokens: %v", err)
	}
	refresher, err := oauth.NewTokenRefresher(oauth.TokenRefresherConfig{
		Service: oauth.NewOAuthService(),
		Store:   store,
		Options: oauth.RefreshTokenOptions{
			TokenURL:   srv.URL + "/token",
			ProfileURL: srv.URL + "/profile",
		},
	})
	if err != nil {
		t.Fatalf("refresher: %v", err)
	}

	client := NewClient(Config{
		AuthToken:      "stale-at",
		BaseURL:        srv.URL,
		HTTPClient:     srv.Client(),
		IsFirstParty:   true,
		TokenRefresher: refresher,
	})

	_, err = client.Stream(context.Background(), minimalRequest())
	if err == nil {
		t.Fatal("expected error when both attempts return 401")
	}
	// After exactly two attempts (initial + one retry) the client must
	// surface the upstream auth error.
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("expected exactly 2 /v1/messages attempts, got %d", got)
	}
}
