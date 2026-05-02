package oauth

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/sheepzhao/claude-code-go/internal/platform/config"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// AuthURLHandler is invoked once the listener is bound and the user-facing
// URLs have been built. The handler receives both the manual URL (always
// safe to copy/paste) and the automatic URL that points at the local
// listener; it is responsible for showing them to the user and, when
// SkipBrowserOpen is false, opening the automatic URL in the user's browser.
type AuthURLHandler func(manualURL, automaticURL string) error

// StartOAuthFlowOptions configures one OAuth login attempt.
type StartOAuthFlowOptions struct {
	// LoginWithClaudeAi selects the Claude.ai authorize URL (instead of the
	// console authorize URL). Forwards to BuildAuthURL.
	LoginWithClaudeAi bool
	// InferenceOnly requests only the `user:inference` scope. Forwards to
	// BuildAuthURL.
	InferenceOnly bool
	// OrganizationUUID, LoginHint, LoginMethod are forwarded verbatim.
	OrganizationUUID string
	LoginHint        string
	LoginMethod      string
	// ExpiresIn, when non-nil, is forwarded to ExchangeCodeForTokens.
	ExpiresIn *int

	// SkipBrowserOpen tells the AuthURLHandler that it should not call
	// openBrowser internally; the caller will display both URLs.
	SkipBrowserOpen bool

	// CallbackPath overrides DefaultCallbackPath on the listener.
	CallbackPath string
	// SuccessRedirectURLs overrides the post-callback browser landing pages.
	SuccessRedirectURLs SuccessRedirectURLs

	// Endpoint URL overrides for tests / staging.
	ConsoleAuthorizeURL  string
	ClaudeAIAuthorizeURL string
	TokenURL             string
	ProfileURL           string
	ManualRedirectURL    string
	ClientID             string

	// HTTPClient, when non-nil, is shared between token-exchange and
	// profile-fetch calls. When nil each call uses its own default client.
	HTTPClient *http.Client
}

// OAuthService orchestrates one Anthropic / Claude.ai OAuth login flow. It
// is a direct port of the TypeScript OAuthService class in
// src/services/oauth/index.ts.
type OAuthService struct {
	mu sync.Mutex

	listener *AuthCodeListener

	// manualCh is non-nil while a flow is in progress. It is closed (or
	// receives a value) when HandleManualAuthCodeInput supplies a code.
	manualCh    chan manualSubmission
	manualState string
}

// manualSubmission carries a user-supplied authorization code.
type manualSubmission struct {
	code  string
	state string
}

// NewOAuthService constructs an empty service. The service is reusable but
// only one flow may be in flight at a time.
func NewOAuthService() *OAuthService {
	return &OAuthService{}
}

// StartOAuthFlow runs the full OAuth login flow once: it boots a localhost
// listener, builds the manual + automatic URLs, hands them to authURLHandler,
// races the captured callback against any HandleManualAuthCodeInput call,
// exchanges the authorization code for tokens, fetches the profile, and
// returns a populated OAuthTokens. Returns an error if any step fails or
// if the context is cancelled.
func (s *OAuthService) StartOAuthFlow(ctx context.Context, opts StartOAuthFlowOptions, authURLHandler AuthURLHandler) (*OAuthTokens, error) {
	if authURLHandler == nil {
		return nil, fmt.Errorf("oauth service: authURLHandler is required")
	}

	codeVerifier, err := GenerateCodeVerifier()
	if err != nil {
		return nil, err
	}
	codeChallenge := GenerateCodeChallenge(codeVerifier)
	state, err := GenerateState()
	if err != nil {
		return nil, err
	}

	listener := NewAuthCodeListener(opts.CallbackPath)
	port, err := listener.Start(0)
	if err != nil {
		return nil, fmt.Errorf("oauth service: start listener: %w", err)
	}

	s.mu.Lock()
	s.listener = listener
	s.manualCh = make(chan manualSubmission, 1)
	s.manualState = state
	manualCh := s.manualCh
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.listener = nil
		s.manualCh = nil
		s.manualState = ""
		s.mu.Unlock()
		_ = listener.Close()
	}()

	urlOpts := OAuthAuthURLOptions{
		CodeChallenge:        codeChallenge,
		State:                state,
		Port:                 port,
		LoginWithClaudeAi:    opts.LoginWithClaudeAi,
		InferenceOnly:        opts.InferenceOnly,
		OrganizationUUID:     opts.OrganizationUUID,
		LoginHint:            opts.LoginHint,
		LoginMethod:          opts.LoginMethod,
		ConsoleAuthorizeURL:  opts.ConsoleAuthorizeURL,
		ClaudeAIAuthorizeURL: opts.ClaudeAIAuthorizeURL,
		ManualRedirectURL:    opts.ManualRedirectURL,
		ClientID:             opts.ClientID,
	}
	urlOpts.IsManual = true
	manualURL, err := BuildAuthURL(urlOpts)
	if err != nil {
		return nil, err
	}
	urlOpts.IsManual = false
	automaticURL, err := BuildAuthURL(urlOpts)
	if err != nil {
		return nil, err
	}

	flowCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	type listenerResult struct {
		code string
		err  error
	}
	listenerCh := make(chan listenerResult, 1)
	go func() {
		code, err := listener.WaitForAuthorization(flowCtx, state, func() error {
			if opts.SkipBrowserOpen {
				return authURLHandler(manualURL, automaticURL)
			}
			return authURLHandler(manualURL, automaticURL)
		})
		listenerCh <- listenerResult{code: code, err: err}
	}()

	var (
		authCode    string
		isAutomatic bool
	)
	select {
	case res := <-listenerCh:
		if res.err != nil {
			return nil, fmt.Errorf("oauth service: wait for callback: %w", res.err)
		}
		authCode = res.code
		isAutomatic = listener.HasPendingResponse()
	case sub := <-manualCh:
		if sub.state != "" && sub.state != state {
			cancel()
			<-listenerCh
			return nil, fmt.Errorf("oauth service: manual state mismatch")
		}
		authCode = sub.code
		isAutomatic = false
		cancel()
		<-listenerCh
	case <-ctx.Done():
		cancel()
		<-listenerCh
		return nil, ctx.Err()
	}

	logger.DebugCF("oauth", "authorization code captured", map[string]any{
		"automatic": isAutomatic,
	})

	tokenResp, err := ExchangeCodeForTokens(ctx, ExchangeCodeForTokensOptions{
		AuthorizationCode: authCode,
		State:             state,
		CodeVerifier:      codeVerifier,
		Port:              port,
		UseManualRedirect: !isAutomatic,
		ExpiresIn:         opts.ExpiresIn,
		TokenURL:          opts.TokenURL,
		ManualRedirectURL: opts.ManualRedirectURL,
		ClientID:          opts.ClientID,
		HTTPClient:        opts.HTTPClient,
	})
	if err != nil {
		if isAutomatic {
			listener.HandleErrorRedirect(opts.SuccessRedirectURLs)
		}
		return nil, err
	}

	profileInfo, profileErr := FetchProfileInfo(ctx, FetchProfileInfoOptions{
		AccessToken: tokenResp.AccessToken,
		ProfileURL:  opts.ProfileURL,
		HTTPClient:  opts.HTTPClient,
	})
	// Profile fetch failure is non-fatal: we still produce OAuthTokens with
	// access/refresh tokens so the caller can persist them. Log the error.
	if profileErr != nil {
		logger.WarnCF("oauth", "profile fetch failed (continuing without profile)", map[string]any{
			"error": profileErr.Error(),
		})
	}

	if isAutomatic {
		scopes := ParseScopes(tokenResp.Scope)
		listener.HandleSuccessRedirect(scopes, opts.SuccessRedirectURLs)
	}

	tokens := formatTokens(tokenResp, profileInfo)
	return tokens, nil
}

// HandleManualAuthCodeInput supplies an authorization code captured outside
// the localhost listener (i.e. the user copy-pasted it from the manual
// success page). state is matched against the in-flight flow when non-empty;
// supplying an empty state skips the check (mirroring the TS reference). It
// is an error to call this method when no flow is in progress.
func (s *OAuthService) HandleManualAuthCodeInput(state, code string) error {
	s.mu.Lock()
	manualCh := s.manualCh
	s.mu.Unlock()
	if manualCh == nil {
		return fmt.Errorf("oauth service: no flow in progress")
	}
	select {
	case manualCh <- manualSubmission{state: state, code: code}:
		return nil
	default:
		return fmt.Errorf("oauth service: manual code already submitted")
	}
}

// Refresh exchanges a refresh token for a new access token, persists the
// result, and optionally fetches updated profile information. It mirrors the
// refreshOAuthToken flow in src/services/oauth/client.ts.
func (s *OAuthService) Refresh(ctx context.Context, refreshToken string, opts RefreshTokenOptions, store *OAuthCredentialStore, writer *config.SettingsWriter) (*OAuthTokens, error) {
	if store == nil {
		return nil, fmt.Errorf("oauth service: credential store is required for refresh")
	}

	tokenResp, err := RefreshToken(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("oauth service: refresh token exchange: %w", err)
	}

	// Attempt to fetch profile info. Failures are non-fatal: we still produce
	// OAuthTokens with the refreshed access/refresh tokens.
	profileInfo, profileErr := FetchProfileInfo(ctx, FetchProfileInfoOptions{
		AccessToken: tokenResp.AccessToken,
		ProfileURL:  opts.ProfileURL,
		HTTPClient:  opts.HTTPClient,
	})
	if profileErr != nil {
		logger.WarnCF("oauth", "profile fetch failed after refresh (continuing without profile)", map[string]any{
			"error": profileErr.Error(),
		})
	}

	tokens := formatTokens(tokenResp, profileInfo)

	decision, saveErr := store.Save(tokens)
	if saveErr != nil {
		return nil, fmt.Errorf("oauth service: persist refreshed credentials: %w", saveErr)
	}
	if !decision.Persisted {
		logger.WarnCF("oauth", "refreshed credentials not persisted", map[string]any{
			"reason": decision.SkipReason,
		})
	}

	if writer != nil && tokens.TokenAccount != nil {
		writeOAuthAccountMetadata(ctx, writer, tokens)
	}

	return tokens, nil
}

// writeOAuthAccountMetadata updates the user-scope settings.json with the
// latest OAuth account metadata. Failures are logged and swallowed.
func writeOAuthAccountMetadata(ctx context.Context, writer *config.SettingsWriter, tokens *OAuthTokens) {
	if writer == nil || tokens == nil {
		return
	}
	updates := map[string]string{
		"oauthAccount.accountUuid":      "",
		"oauthAccount.emailAddress":     "",
		"oauthAccount.organizationUuid": "",
		"oauthAccount.organizationName": "",
	}
	if tokens.TokenAccount != nil {
		updates["oauthAccount.accountUuid"] = tokens.TokenAccount.UUID
		updates["oauthAccount.emailAddress"] = tokens.TokenAccount.EmailAddress
		updates["oauthAccount.organizationUuid"] = tokens.TokenAccount.OrganizationUUID
	}
	if tokens.Profile != nil && tokens.Profile.Account.Email != "" {
		updates["oauthAccount.emailAddress"] = tokens.Profile.Account.Email
	}
	for key, value := range updates {
		if value == "" {
			continue
		}
		if err := writer.Set(ctx, "user", key, value); err != nil {
			logger.WarnCF("oauth", "failed to update oauthAccount metadata after refresh", map[string]any{
				"key":   key,
				"error": err.Error(),
			})
		}
	}
}

// Cleanup tears down any in-flight flow. It is safe to call multiple times.
func (s *OAuthService) Cleanup() error {
	s.mu.Lock()
	listener := s.listener
	s.listener = nil
	s.manualCh = nil
	s.manualState = ""
	s.mu.Unlock()
	if listener == nil {
		return nil
	}
	return listener.Close()
}

// formatTokens projects the token-exchange response and profile info into the
// caller-facing OAuthTokens struct. Mirrors formatTokens() in
// src/services/oauth/index.ts.
func formatTokens(resp *OAuthTokenExchangeResponse, profile *ProfileInfo) *OAuthTokens {
	tokens := &OAuthTokens{
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
		ExpiresAt:    nowMillis() + int64(resp.ExpiresIn)*1000,
		Scopes:       ParseScopes(resp.Scope),
	}
	if profile != nil {
		tokens.SubscriptionType = profile.SubscriptionType
		tokens.RateLimitTier = profile.RateLimitTier
		tokens.HasExtraUsageEnabled = profile.HasExtraUsageEnabled
		tokens.BillingType = profile.BillingType
		tokens.Profile = &profile.RawProfile
	}
	if resp.Account != nil {
		info := &OAuthTokenAccountInfo{
			UUID:         resp.Account.UUID,
			EmailAddress: resp.Account.EmailAddress,
		}
		if resp.Organization != nil {
			info.OrganizationUUID = resp.Organization.UUID
		}
		tokens.TokenAccount = info
	}
	return tokens
}
