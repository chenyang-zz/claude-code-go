package oauth

import (
	"errors"
	"fmt"
)

// ErrOAuthRefreshFailed is returned by the auto-refresh path when an OAuth
// refresh attempt fails terminally and the user must re-run /login.
//
// Callers can match it with errors.Is to surface a uniform "please log in
// again" message regardless of the underlying transport error.
var ErrOAuthRefreshFailed = errors.New("oauth refresh failed; please run /login")

// OAuthTokenExpiredError represents an authentication failure observed at the
// HTTP layer that the OAuth refresher should react to. It carries the failed
// access token so that callers can correlate the failure with an in-flight
// dedup key.
type OAuthTokenExpiredError struct {
	// Status is the HTTP status code that triggered the classification
	// (typically 401).
	Status int
	// FailedAccessToken is the bearer token whose use produced the auth
	// failure. It is used as the in-flight dedup key when multiple
	// concurrent requests share the same expired token.
	FailedAccessToken string
}

// Error implements the error interface.
func (e *OAuthTokenExpiredError) Error() string {
	return fmt.Sprintf("oauth token expired (status=%d)", e.Status)
}

// IsOAuthAuthError returns true when the supplied HTTP status code indicates
// an OAuth authentication failure that the refresher should attempt to
// recover from. Mirrors the primary branch of `withOAuth401Retry` in
// src/utils/http.ts which treats HTTP 401 as the canonical signal.
//
// The TS-side `also403Revoked` extension that classifies 403 + body containing
// "OAuth token has been revoked" is intentionally not recreated in this batch.
func IsOAuthAuthError(status int) bool {
	return status == 401
}
