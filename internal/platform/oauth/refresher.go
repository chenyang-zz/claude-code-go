package oauth

import (
	"context"
	"errors"
	"fmt"

	"github.com/sheepzhao/claude-code-go/internal/platform/config"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// preemptiveRefreshKey is the dedup key used by the preemptive refresh path.
// It mirrors the TS singleton `pendingRefreshCheck` behavior in
// src/utils/auth.ts:1425, where preemptive checks share a single in-flight
// promise rather than fanning out per token.
const preemptiveRefreshKey = "__preemptive__"

// TokenRefresher coordinates OAuth credential refresh on behalf of HTTP
// clients that consume bearer tokens. It exposes two paths:
//
//   - MaybeRefresh: preemptive — invoked before each request to refresh
//     tokens that are within OAuthRefreshBuffer of expiry.
//   - Refresh: reactive — invoked when a request observed an HTTP 401 so
//     that the access token can be rotated and the request retried once.
//
// Both paths funnel through a shared RefreshDeduper so that concurrent
// callers cannot trigger multiple refreshes for the same token.
type TokenRefresher struct {
	service *OAuthService
	store   *OAuthCredentialStore
	writer  *config.SettingsWriter
	deduper *RefreshDeduper

	// options is the base set of RefreshTokenOptions fed into
	// OAuthService.Refresh on every refresh attempt. Callers populate the
	// fields that vary between environments (TokenURL, ClientID, HTTPClient).
	options RefreshTokenOptions
}

// TokenRefresherConfig captures the construction-time dependencies of a
// TokenRefresher. The Service and Store fields are required; Writer is
// optional but recommended so the user-scope settings.json oauthAccount
// metadata stays in sync after every refresh.
type TokenRefresherConfig struct {
	Service *OAuthService
	Store   *OAuthCredentialStore
	Writer  *config.SettingsWriter

	// Options carries the refresh request defaults: TokenURL, ClientID,
	// Scopes, and HTTPClient. RefreshToken is overwritten per call.
	Options RefreshTokenOptions
}

// NewTokenRefresher builds a TokenRefresher with its own RefreshDeduper.
// The Service and Store must be non-nil.
func NewTokenRefresher(cfg TokenRefresherConfig) (*TokenRefresher, error) {
	if cfg.Service == nil {
		return nil, fmt.Errorf("oauth: token refresher requires a service")
	}
	if cfg.Store == nil {
		return nil, fmt.Errorf("oauth: token refresher requires a credential store")
	}
	return &TokenRefresher{
		service: cfg.Service,
		store:   cfg.Store,
		writer:  cfg.Writer,
		deduper: NewRefreshDeduper(),
		options: cfg.Options,
	}, nil
}

// CurrentTokens returns the most recently persisted OAuth tokens, or nil if
// the credential store has no entry. It does not consult any in-memory
// cache because the underlying store is the source of truth.
func (r *TokenRefresher) CurrentTokens() (*OAuthTokens, error) {
	tokens, err := r.store.Load()
	if err != nil {
		return nil, fmt.Errorf("oauth refresher: load credentials: %w", err)
	}
	return tokens, nil
}

// MaybeRefresh inspects the persisted credentials and refreshes them when
// they are within OAuthRefreshBuffer of expiry. When the tokens are still
// fresh it returns them unchanged so callers can use the existing access
// token for the upcoming request.
//
// If no credentials are stored, or the stored credentials lack a refresh
// token, MaybeRefresh returns (nil, nil) and the caller should fall back to
// whatever credential mode it was configured with (e.g. API key).
func (r *TokenRefresher) MaybeRefresh(ctx context.Context) (*OAuthTokens, error) {
	tokens, err := r.CurrentTokens()
	if err != nil {
		return nil, err
	}
	if tokens == nil {
		return nil, nil
	}
	if tokens.RefreshToken == "" {
		return tokens, nil
	}
	if !IsOAuthTokenExpired(tokens.ExpiresAt) {
		return tokens, nil
	}
	return r.runRefresh(ctx, preemptiveRefreshKey, tokens.RefreshToken)
}

// Refresh forces an OAuth refresh in response to an upstream 401. The
// failedAccessToken is used as the dedup key so that N concurrent requests
// that observed 401 with the same token only trigger one refresh.
//
// Before issuing the refresh, Refresh re-reads the credential store. If the
// stored access token has already changed (because another goroutine or
// process refreshed first), the freshly persisted tokens are returned
// without contacting the server again.
func (r *TokenRefresher) Refresh(ctx context.Context, failedAccessToken string) (*OAuthTokens, error) {
	if failedAccessToken == "" {
		return nil, fmt.Errorf("oauth refresher: failed access token is required")
	}

	tokens, err := r.CurrentTokens()
	if err != nil {
		return nil, err
	}
	if tokens == nil || tokens.RefreshToken == "" {
		return nil, fmt.Errorf("%w: no refresh token available", ErrOAuthRefreshFailed)
	}

	// Another caller already rotated the token (different access token in
	// the store than the one that produced the 401) — reuse it.
	if tokens.AccessToken != failedAccessToken {
		logger.DebugCF("oauth_refresher", "reusing freshly refreshed credentials from store", map[string]any{
			"reason": "store_already_rotated",
		})
		return tokens, nil
	}

	return r.runRefresh(ctx, failedAccessToken, tokens.RefreshToken)
}

// runRefresh performs the actual server round-trip while holding the dedup
// slot for `key`. Concurrent callers with the same key wait for this
// invocation and observe its result.
func (r *TokenRefresher) runRefresh(ctx context.Context, key, refreshToken string) (*OAuthTokens, error) {
	return r.deduper.Do(key, func() (*OAuthTokens, error) {
		opts := r.options
		opts.RefreshToken = refreshToken

		tokens, err := r.service.Refresh(ctx, refreshToken, opts, r.store, r.writer)
		if err != nil {
			logger.WarnCF("oauth_refresher", "refresh attempt failed", map[string]any{
				"key":   key,
				"error": err.Error(),
			})
			return nil, fmt.Errorf("%w: %s", ErrOAuthRefreshFailed, err.Error())
		}
		logger.DebugCF("oauth_refresher", "refresh succeeded", map[string]any{
			"key": key,
		})
		return tokens, nil
	})
}

// HandleFailure normalizes a refresh error for upstream callers. It always
// returns an error chain that satisfies errors.Is(err, ErrOAuthRefreshFailed)
// so command surfaces can render a single "please run /login" message.
//
// HandleFailure does NOT delete the persisted credentials. The TS reference
// only clears credentials on explicit /logout, and the server may transiently
// reject a refresh attempt for reasons other than revocation (network blip,
// rate limit), so deleting on the first failure would lose recoverable
// tokens. Users explicitly re-authenticate via /login.
func (r *TokenRefresher) HandleFailure(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrOAuthRefreshFailed) {
		return err
	}
	return fmt.Errorf("%w: %s", ErrOAuthRefreshFailed, err.Error())
}
