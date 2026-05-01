package oauth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// seedExpiredTokens persists a token bundle whose ExpiresAt is already in the
// past, so MaybeRefresh treats it as expired.
func seedExpiredTokens(t *testing.T, store *OAuthCredentialStore, accessToken, refreshToken string) {
	t.Helper()
	tokens := &OAuthTokens{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    time.Now().Add(-1 * time.Minute).UnixMilli(),
		Scopes:       []string{"user:inference", "user:profile"},
	}
	decision, err := store.Save(tokens)
	if err != nil {
		t.Fatalf("seed expired tokens: %v", err)
	}
	if !decision.Persisted {
		t.Fatalf("expected expired tokens to persist; skip reason: %s", decision.SkipReason)
	}
}

// seedFreshTokens persists a token bundle whose ExpiresAt is well beyond the
// 5 minute buffer so MaybeRefresh treats it as still valid.
func seedFreshTokens(t *testing.T, store *OAuthCredentialStore, accessToken, refreshToken string) {
	t.Helper()
	tokens := &OAuthTokens{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    time.Now().Add(1 * time.Hour).UnixMilli(),
		Scopes:       []string{"user:inference", "user:profile"},
	}
	decision, err := store.Save(tokens)
	if err != nil {
		t.Fatalf("seed fresh tokens: %v", err)
	}
	if !decision.Persisted {
		t.Fatalf("expected fresh tokens to persist; skip reason: %s", decision.SkipReason)
	}
}

// startUpstream spins up an httptest server hosting the canned OAuth token +
// profile endpoints used by the refresher tests.
func startUpstream(t *testing.T) (*fakeUpstream, *httptest.Server) {
	t.Helper()
	upstream := newFakeUpstream()
	srv := httptest.NewServer(upstream.handler())
	t.Cleanup(srv.Close)
	return upstream, srv
}

func newRefresherWithUpstream(t *testing.T, srv *httptest.Server) (*TokenRefresher, *OAuthCredentialStore) {
	t.Helper()
	store, err := NewOAuthCredentialStore(t.TempDir())
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	refresher, err := NewTokenRefresher(TokenRefresherConfig{
		Service: NewOAuthService(),
		Store:   store,
		Options: RefreshTokenOptions{
			TokenURL:   srv.URL + "/token",
			ProfileURL: srv.URL + "/profile",
		},
	})
	if err != nil {
		t.Fatalf("create refresher: %v", err)
	}
	return refresher, store
}

func TestTokenRefresher_MaybeRefresh_NoStoredTokens(t *testing.T) {
	store, err := NewOAuthCredentialStore(t.TempDir())
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	refresher, err := NewTokenRefresher(TokenRefresherConfig{
		Service: NewOAuthService(),
		Store:   store,
	})
	if err != nil {
		t.Fatalf("create refresher: %v", err)
	}
	tokens, err := refresher.MaybeRefresh(context.Background())
	if err != nil {
		t.Fatalf("MaybeRefresh: %v", err)
	}
	if tokens != nil {
		t.Fatalf("expected nil tokens for empty store, got %+v", tokens)
	}
}

func TestTokenRefresher_MaybeRefresh_FreshTokensSkipServer(t *testing.T) {
	upstream, srv := startUpstream(t)
	refresher, store := newRefresherWithUpstream(t, srv)
	seedFreshTokens(t, store, "current-at", "current-rt")

	tokens, err := refresher.MaybeRefresh(context.Background())
	if err != nil {
		t.Fatalf("MaybeRefresh: %v", err)
	}
	if tokens == nil || tokens.AccessToken != "current-at" {
		t.Fatalf("expected fresh tokens returned unchanged, got %+v", tokens)
	}
	if got := atomic.LoadInt32(&upstream.tokenCallCount); got != 0 {
		t.Fatalf("expected 0 token calls for fresh credentials, got %d", got)
	}
}

func TestTokenRefresher_MaybeRefresh_ExpiredTokensTriggerRefresh(t *testing.T) {
	upstream, srv := startUpstream(t)
	refresher, store := newRefresherWithUpstream(t, srv)
	seedExpiredTokens(t, store, "old-at", "old-rt")

	tokens, err := refresher.MaybeRefresh(context.Background())
	if err != nil {
		t.Fatalf("MaybeRefresh: %v", err)
	}
	if tokens == nil || tokens.AccessToken != "fake-at" {
		t.Fatalf("expected refreshed access token, got %+v", tokens)
	}
	if got := atomic.LoadInt32(&upstream.tokenCallCount); got != 1 {
		t.Fatalf("expected exactly 1 token call, got %d", got)
	}
	stored, err := store.Load()
	if err != nil {
		t.Fatalf("store.Load: %v", err)
	}
	if stored == nil || stored.AccessToken != "fake-at" {
		t.Fatalf("store should contain refreshed tokens, got %+v", stored)
	}
}

func TestTokenRefresher_Refresh_DetectsAlreadyRotatedToken(t *testing.T) {
	upstream, srv := startUpstream(t)
	refresher, store := newRefresherWithUpstream(t, srv)
	// Persist tokens whose AccessToken differs from the one that "failed" —
	// simulating a sibling goroutine that already rotated the credential.
	seedFreshTokens(t, store, "rotated-at", "rotated-rt")

	tokens, err := refresher.Refresh(context.Background(), "stale-at-that-was-rejected")
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if tokens == nil || tokens.AccessToken != "rotated-at" {
		t.Fatalf("expected rotated tokens reused, got %+v", tokens)
	}
	if got := atomic.LoadInt32(&upstream.tokenCallCount); got != 0 {
		t.Fatalf("expected 0 token calls when store already rotated, got %d", got)
	}
}

func TestTokenRefresher_Refresh_NoCredentialsReturnsRefreshFailed(t *testing.T) {
	store, err := NewOAuthCredentialStore(t.TempDir())
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	refresher, err := NewTokenRefresher(TokenRefresherConfig{
		Service: NewOAuthService(),
		Store:   store,
	})
	if err != nil {
		t.Fatalf("create refresher: %v", err)
	}
	_, err = refresher.Refresh(context.Background(), "stale")
	if !errors.Is(err, ErrOAuthRefreshFailed) {
		t.Fatalf("expected ErrOAuthRefreshFailed, got %v", err)
	}
}

func TestTokenRefresher_Refresh_PerformsServerRoundTripWhenSameToken(t *testing.T) {
	upstream, srv := startUpstream(t)
	refresher, store := newRefresherWithUpstream(t, srv)
	seedFreshTokens(t, store, "stale-at", "stale-rt")

	tokens, err := refresher.Refresh(context.Background(), "stale-at")
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if tokens == nil || tokens.AccessToken != "fake-at" {
		t.Fatalf("expected new access token, got %+v", tokens)
	}
	if got := atomic.LoadInt32(&upstream.tokenCallCount); got != 1 {
		t.Fatalf("expected 1 token call, got %d", got)
	}
}

func TestTokenRefresher_Refresh_ServerFailureWrapsErrOAuthRefreshFailed(t *testing.T) {
	upstream := newFakeUpstream()
	upstream.tokenStatus = http.StatusInternalServerError
	upstream.tokenBody = `{"error":"upstream boom"}`
	srv := httptest.NewServer(upstream.handler())
	t.Cleanup(srv.Close)

	refresher, store := newRefresherWithUpstream(t, srv)
	seedExpiredTokens(t, store, "old-at", "old-rt")

	_, err := refresher.MaybeRefresh(context.Background())
	if err == nil {
		t.Fatal("expected error from upstream 500")
	}
	if !errors.Is(err, ErrOAuthRefreshFailed) {
		t.Fatalf("expected ErrOAuthRefreshFailed in chain, got %v", err)
	}
	// The failure path must NOT delete the persisted credentials.
	stored, err := store.Load()
	if err != nil {
		t.Fatalf("store.Load after failure: %v", err)
	}
	if stored == nil || stored.AccessToken != "old-at" {
		t.Fatalf("expected credentials retained after refresh failure, got %+v", stored)
	}
}

func TestTokenRefresher_HandleFailure_WrapsArbitraryError(t *testing.T) {
	store, err := NewOAuthCredentialStore(t.TempDir())
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	refresher, err := NewTokenRefresher(TokenRefresherConfig{
		Service: NewOAuthService(),
		Store:   store,
	})
	if err != nil {
		t.Fatalf("create refresher: %v", err)
	}
	if got := refresher.HandleFailure(nil); got != nil {
		t.Fatalf("expected nil for nil input, got %v", got)
	}
	wrapped := refresher.HandleFailure(errors.New("network timeout"))
	if !errors.Is(wrapped, ErrOAuthRefreshFailed) {
		t.Fatalf("expected wrapping with ErrOAuthRefreshFailed, got %v", wrapped)
	}
	// Already-wrapped errors should pass through unchanged.
	pre := refresher.HandleFailure(ErrOAuthRefreshFailed)
	if pre != ErrOAuthRefreshFailed {
		t.Fatalf("expected sentinel passthrough, got %v", pre)
	}
}
