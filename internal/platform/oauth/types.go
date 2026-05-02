package oauth

import "time"

// nowMillis returns the current Unix time in milliseconds, mirroring the
// JavaScript `Date.now()` semantics used by the TypeScript reference.
func nowMillis() int64 {
	return time.Now().UnixMilli()
}

// SubscriptionType represents the high-level subscription tier returned by
// the Claude.ai profile endpoint. Mirrors the union type on the TS side
// (`'max' | 'pro' | 'enterprise' | 'team'`) but is treated as an open string
// so that unknown values from a future server can flow through unchanged.
type SubscriptionType string

const (
	// SubscriptionMax is mapped from `claude_max`.
	SubscriptionMax SubscriptionType = "max"
	// SubscriptionPro is mapped from `claude_pro`.
	SubscriptionPro SubscriptionType = "pro"
	// SubscriptionEnterprise is mapped from `claude_enterprise`.
	SubscriptionEnterprise SubscriptionType = "enterprise"
	// SubscriptionTeam is mapped from `claude_team`.
	SubscriptionTeam SubscriptionType = "team"
)

// RateLimitTier is a string that mirrors the same field on the TS side. It is
// kept open because the server may return new tier names independent of CLI
// releases (e.g. `default_claude_max_5x`).
type RateLimitTier string

// BillingType is the open billing-type identifier returned by the profile
// endpoint. Stored verbatim and surfaced via /status.
type BillingType string

// OAuthAuthURLOptions captures the inputs to BuildAuthURL.
type OAuthAuthURLOptions struct {
	// CodeChallenge is the base64url-encoded sha256 PKCE challenge.
	CodeChallenge string
	// State is the CSRF state token validated on the callback.
	State string
	// Port is the port the local callback listener is bound to. It is only
	// used when IsManual is false.
	Port int
	// LoginWithClaudeAi selects the Claude.ai authorization endpoint when
	// true, falling back to the console endpoint otherwise.
	LoginWithClaudeAi bool
	// IsManual selects the manual redirect URI (a hosted page) when true,
	// allowing users to copy the authorization code by hand. When false the
	// redirect URI points at the local callback listener.
	IsManual bool
	// InferenceOnly requests only the `user:inference` scope. When false
	// the full ALL_OAUTH_SCOPES set is requested.
	InferenceOnly bool
	// OrganizationUUID, when set, is forwarded as `organization_uuid` so
	// that the authorization page selects the correct org.
	OrganizationUUID string
	// LoginHint, when set, is forwarded as `login_hint` for SSO routing.
	LoginHint string
	// LoginMethod, when set, is forwarded as `login_method`.
	LoginMethod string

	// ConsoleAuthorizeURL overrides DefaultConsoleAuthorizeURL.
	ConsoleAuthorizeURL string
	// ClaudeAIAuthorizeURL overrides DefaultClaudeAIAuthorizeURL.
	ClaudeAIAuthorizeURL string
	// ManualRedirectURL overrides DefaultManualRedirectURL.
	ManualRedirectURL string
	// ClientID overrides DefaultClientID.
	ClientID string
}

// OAuthTokenExchangeRequest is the JSON body POSTed to the OAuth token
// endpoint. Field tags mirror the TypeScript reference exactly.
type OAuthTokenExchangeRequest struct {
	GrantType    string `json:"grant_type"`
	Code         string `json:"code"`
	RedirectURI  string `json:"redirect_uri"`
	ClientID     string `json:"client_id"`
	CodeVerifier string `json:"code_verifier"`
	State        string `json:"state"`
	ExpiresIn    *int   `json:"expires_in,omitempty"`
}

// OAuthTokenAccount represents the `account` block embedded in the token
// exchange response. Both fields may be empty.
type OAuthTokenAccount struct {
	UUID         string `json:"uuid,omitempty"`
	EmailAddress string `json:"email_address,omitempty"`
}

// OAuthTokenOrganization represents the `organization` block embedded in the
// token exchange response.
type OAuthTokenOrganization struct {
	UUID string `json:"uuid,omitempty"`
}

// OAuthTokenExchangeResponse mirrors the JSON returned by the token endpoint.
type OAuthTokenExchangeResponse struct {
	AccessToken  string                  `json:"access_token"`
	RefreshToken string                  `json:"refresh_token,omitempty"`
	ExpiresIn    int                     `json:"expires_in"`
	Scope        string                  `json:"scope,omitempty"`
	Account      *OAuthTokenAccount      `json:"account,omitempty"`
	Organization *OAuthTokenOrganization `json:"organization,omitempty"`
}

// OAuthProfileAccount mirrors the `account` block from the profile endpoint.
type OAuthProfileAccount struct {
	UUID        string `json:"uuid,omitempty"`
	Email       string `json:"email,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
}

// OAuthProfileOrganization mirrors the `organization` block from the profile
// endpoint.
type OAuthProfileOrganization struct {
	UUID                  string `json:"uuid,omitempty"`
	OrganizationType      string `json:"organization_type,omitempty"`
	RateLimitTier         string `json:"rate_limit_tier,omitempty"`
	HasExtraUsageEnabled  bool   `json:"has_extra_usage_enabled,omitempty"`
	BillingType           string `json:"billing_type,omitempty"`
	SubscriptionCreatedAt string `json:"subscription_created_at,omitempty"`
}

// OAuthProfileResponse is the parsed JSON body returned by the profile endpoint.
type OAuthProfileResponse struct {
	Account      OAuthProfileAccount      `json:"account"`
	Organization OAuthProfileOrganization `json:"organization"`
}

// ProfileInfo is the normalized projection produced by FetchProfileInfo.
type ProfileInfo struct {
	// SubscriptionType is the mapped tier (max/pro/enterprise/team) or empty
	// when the server returned an unknown organization type.
	SubscriptionType SubscriptionType
	// RateLimitTier echoes organization.rate_limit_tier verbatim.
	RateLimitTier RateLimitTier
	// DisplayName is taken from account.display_name.
	DisplayName string
	// HasExtraUsageEnabled is taken from organization.has_extra_usage_enabled.
	HasExtraUsageEnabled bool
	// BillingType echoes organization.billing_type verbatim.
	BillingType BillingType
	// AccountCreatedAt is taken from account.created_at.
	AccountCreatedAt string
	// SubscriptionCreatedAt is taken from organization.subscription_created_at.
	SubscriptionCreatedAt string
	// RawProfile is the original profile envelope, retained so that callers
	// can persist whatever fields they care about.
	RawProfile OAuthProfileResponse
}

// OAuthTokens is the final structure returned to the caller of an OAuth flow.
// It carries everything needed for both the credential store and the
// `oauthAccount` settings update.
type OAuthTokens struct {
	AccessToken      string
	RefreshToken     string
	ExpiresAt        int64 // Unix milliseconds, matching the TS reference.
	Scopes           []string
	SubscriptionType SubscriptionType
	RateLimitTier    RateLimitTier
	// HasExtraUsageEnabled mirrors organization.has_extra_usage_enabled from
	// the profile endpoint. Persisted alongside the credentials so callers
	// outside the OAuth flow (e.g. rate limit upsell text) can branch on it
	// without re-fetching the profile.
	HasExtraUsageEnabled bool
	// BillingType mirrors organization.billing_type. Persisted to support
	// overage provisioning gating; an empty string means the tier cannot be
	// determined yet.
	BillingType BillingType
	Profile      *OAuthProfileResponse
	TokenAccount *OAuthTokenAccountInfo
}

// OAuthTokenAccountInfo is the normalized projection of OAuthTokenExchangeResponse.Account
// (and parent OAuthTokenExchangeResponse.Organization) used by callers.
type OAuthTokenAccountInfo struct {
	UUID             string
	EmailAddress     string
	OrganizationUUID string
}
