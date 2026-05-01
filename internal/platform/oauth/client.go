package oauth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// tokenExchangeTimeout matches the 15-second timeout used by the TypeScript
// reference (`src/services/oauth/client.ts`).
const tokenExchangeTimeout = 15 * time.Second

// callbackLocalhostPath is the path appended to the localhost redirect URI
// when the listener is bound. It matches DefaultCallbackPath but is kept as a
// separate constant to make it explicit at the URL-construction site.
const callbackLocalhostPath = DefaultCallbackPath

// ParseScopes splits a space-separated scope string and discards empty
// fragments. Mirrors parseScopes() in src/services/oauth/client.ts.
func ParseScopes(scopeString string) []string {
	if strings.TrimSpace(scopeString) == "" {
		return nil
	}
	parts := strings.Split(scopeString, " ")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			out = append(out, p)
		}
	}
	return out
}

// ShouldUseClaudeAIAuth reports whether the granted scopes include the
// Claude.ai inference scope (`user:inference`). Mirrors shouldUseClaudeAIAuth().
func ShouldUseClaudeAIAuth(scopes []string) bool {
	for _, s := range scopes {
		if s == ScopeUserInference {
			return true
		}
	}
	return false
}

// BuildAuthURL renders the user-facing OAuth authorization URL. The selection
// of base endpoint, redirect_uri, and scope set follows the TypeScript
// reference implementation in src/services/oauth/client.ts.
func BuildAuthURL(opts OAuthAuthURLOptions) (string, error) {
	base := opts.ConsoleAuthorizeURL
	if opts.LoginWithClaudeAi {
		base = opts.ClaudeAIAuthorizeURL
		if base == "" {
			base = DefaultClaudeAIAuthorizeURL
		}
	} else if base == "" {
		base = DefaultConsoleAuthorizeURL
	}

	parsed, err := url.Parse(base)
	if err != nil {
		return "", fmt.Errorf("oauth: parse authorize URL %q: %w", base, err)
	}

	clientID := opts.ClientID
	if clientID == "" {
		clientID = DefaultClientID
	}

	redirectURI := redirectURIFor(opts)
	scopeString := buildScopeString(opts.InferenceOnly)

	q := parsed.Query()
	q.Set("code", "true")
	q.Set("client_id", clientID)
	q.Set("response_type", "code")
	q.Set("redirect_uri", redirectURI)
	q.Set("scope", scopeString)
	q.Set("code_challenge", opts.CodeChallenge)
	q.Set("code_challenge_method", "S256")
	q.Set("state", opts.State)
	if opts.OrganizationUUID != "" {
		q.Set("organization_uuid", opts.OrganizationUUID)
	}
	if opts.LoginHint != "" {
		q.Set("login_hint", opts.LoginHint)
	}
	if opts.LoginMethod != "" {
		q.Set("login_method", opts.LoginMethod)
	}
	parsed.RawQuery = q.Encode()
	return parsed.String(), nil
}

// redirectURIFor returns the redirect_uri appropriate for the requested flow.
func redirectURIFor(opts OAuthAuthURLOptions) string {
	if opts.IsManual {
		if opts.ManualRedirectURL != "" {
			return opts.ManualRedirectURL
		}
		return DefaultManualRedirectURL
	}
	port := opts.Port
	if port <= 0 {
		port = 0
	}
	return fmt.Sprintf("http://localhost:%d%s", port, callbackLocalhostPath)
}

// buildScopeString joins the scope set into a space-separated query string.
func buildScopeString(inferenceOnly bool) string {
	if inferenceOnly {
		return ScopeUserInference
	}
	return strings.Join(DefaultAllScopes, " ")
}

// ExchangeCodeForTokensOptions captures the inputs to ExchangeCodeForTokens.
type ExchangeCodeForTokensOptions struct {
	// AuthorizationCode is the `code` value returned by the OAuth provider.
	AuthorizationCode string
	// State is the original CSRF state (the server echoes it back; we send
	// it along on the exchange so the upstream can rebind the session).
	State string
	// CodeVerifier is the PKCE verifier whose challenge was sent to the
	// authorize endpoint.
	CodeVerifier string
	// Port is the port the local callback listener was bound to. It is only
	// used when UseManualRedirect is false.
	Port int
	// UseManualRedirect selects the manual redirect_uri when true. Must be
	// set to true if and only if the authorization code was captured via
	// the manual paste path (not via the localhost listener).
	UseManualRedirect bool
	// ExpiresIn, when non-nil, requests a custom token TTL.
	ExpiresIn *int

	// TokenURL overrides DefaultTokenURL.
	TokenURL string
	// ManualRedirectURL overrides DefaultManualRedirectURL.
	ManualRedirectURL string
	// ClientID overrides DefaultClientID.
	ClientID string
	// HTTPClient, when non-nil, is used to issue the token-exchange POST.
	// When nil a fresh http.Client with Timeout=15s is used.
	HTTPClient *http.Client
}

// ExchangeCodeForTokens posts the authorization code to the OAuth token
// endpoint and parses the response. It returns a structured error when the
// server replies with 401 (so callers can surface "invalid authorization
// code") and a generic error for any other non-2xx status.
func ExchangeCodeForTokens(ctx context.Context, opts ExchangeCodeForTokensOptions) (*OAuthTokenExchangeResponse, error) {
	tokenURL := opts.TokenURL
	if tokenURL == "" {
		tokenURL = DefaultTokenURL
	}
	clientID := opts.ClientID
	if clientID == "" {
		clientID = DefaultClientID
	}
	manualURL := opts.ManualRedirectURL
	if manualURL == "" {
		manualURL = DefaultManualRedirectURL
	}

	redirectURI := manualURL
	if !opts.UseManualRedirect {
		redirectURI = fmt.Sprintf("http://localhost:%d%s", opts.Port, callbackLocalhostPath)
	}

	body := OAuthTokenExchangeRequest{
		GrantType:    "authorization_code",
		Code:         opts.AuthorizationCode,
		RedirectURI:  redirectURI,
		ClientID:     clientID,
		CodeVerifier: opts.CodeVerifier,
		State:        opts.State,
		ExpiresIn:    opts.ExpiresIn,
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("oauth: marshal token exchange request: %w", err)
	}

	httpClient := opts.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: tokenExchangeTimeout}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("oauth: build token exchange request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("anthropic-beta", OAuthBetaHeader)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("oauth: token exchange POST: %w", err)
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("oauth: read token exchange response: %w", err)
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("oauth: authentication failed: invalid authorization code")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("oauth: token exchange failed (status %d %s)", resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	var parsed OAuthTokenExchangeResponse
	if err := json.Unmarshal(rawBody, &parsed); err != nil {
		return nil, fmt.Errorf("oauth: decode token exchange response: %w", err)
	}
	if strings.TrimSpace(parsed.AccessToken) == "" {
		return nil, fmt.Errorf("oauth: token exchange response missing access_token")
	}

	logger.DebugCF("oauth", "token exchange succeeded", map[string]any{
		"manual":            opts.UseManualRedirect,
		"scope_count":       len(ParseScopes(parsed.Scope)),
		"has_refresh_token": parsed.RefreshToken != "",
		"expires_in":        parsed.ExpiresIn,
	})
	return &parsed, nil
}

// RefreshTokenOptions captures the inputs to RefreshToken.
type RefreshTokenOptions struct {
	// RefreshToken is the existing refresh token.
	RefreshToken string
	// Scopes, when non-empty, overrides the default scope set sent to the
	// token endpoint. When empty DefaultClaudeAIScopes is used.
	Scopes []string
	// TokenURL overrides DefaultTokenURL.
	TokenURL string
	// ClientID overrides DefaultClientID.
	ClientID string
	// HTTPClient, when non-nil, is used to issue the refresh POST.
	// When nil a fresh http.Client with Timeout=15s is used.
	HTTPClient *http.Client
}

// RefreshToken posts a refresh_token grant to the OAuth token endpoint and
// parses the response. It mirrors refreshOAuthToken() in
// src/services/oauth/client.ts.
func RefreshToken(ctx context.Context, opts RefreshTokenOptions) (*OAuthTokenExchangeResponse, error) {
	if strings.TrimSpace(opts.RefreshToken) == "" {
		return nil, fmt.Errorf("oauth: refresh token is empty")
	}

	tokenURL := opts.TokenURL
	if tokenURL == "" {
		tokenURL = DefaultTokenURL
	}
	clientID := opts.ClientID
	if clientID == "" {
		clientID = DefaultClientID
	}

	scopes := opts.Scopes
	if len(scopes) == 0 {
		scopes = DefaultClaudeAIScopes
	}

	body := map[string]string{
		"grant_type":    "refresh_token",
		"refresh_token": opts.RefreshToken,
		"client_id":     clientID,
		"scope":         strings.Join(scopes, " "),
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("oauth: marshal refresh token request: %w", err)
	}

	httpClient := opts.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: tokenExchangeTimeout}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("oauth: build refresh token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("oauth: refresh token POST: %w", err)
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("oauth: read refresh token response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("oauth: token refresh failed (status %d %s)", resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	var parsed OAuthTokenExchangeResponse
	if err := json.Unmarshal(rawBody, &parsed); err != nil {
		return nil, fmt.Errorf("oauth: decode refresh token response: %w", err)
	}
	if strings.TrimSpace(parsed.AccessToken) == "" {
		return nil, fmt.Errorf("oauth: refresh token response missing access_token")
	}
	// Fallback: if the server did not return a new refresh_token, preserve
	// the original one so callers can continue using it.
	if parsed.RefreshToken == "" {
		parsed.RefreshToken = opts.RefreshToken
	}

	logger.DebugCF("oauth", "token refresh succeeded", map[string]any{
		"scope_count":       len(ParseScopes(parsed.Scope)),
		"has_refresh_token": parsed.RefreshToken != "",
		"expires_in":        parsed.ExpiresIn,
	})
	return &parsed, nil
}
