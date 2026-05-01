package oauth

// Production OAuth endpoints used by the Anthropic / Claude.ai login flow.
// These mirror PROD_OAUTH_CONFIG in src/constants/oauth.ts. Staging, local,
// and CLAUDE_CODE_CUSTOM_OAUTH_URL overrides are intentionally not migrated.

// DefaultBaseAPIURL is the Anthropic API base URL.
const DefaultBaseAPIURL = "https://api.anthropic.com"

// DefaultConsoleAuthorizeURL is the OAuth authorization endpoint for
// console-only logins.
const DefaultConsoleAuthorizeURL = "https://platform.claude.com/oauth/authorize"

// DefaultClaudeAIAuthorizeURL is the OAuth authorization endpoint for
// Claude.ai logins (the URL users see when picking the Claude Max upsell).
const DefaultClaudeAIAuthorizeURL = "https://claude.com/cai/oauth/authorize"

// DefaultTokenURL is the OAuth token exchange endpoint.
const DefaultTokenURL = "https://platform.claude.com/v1/oauth/token"

// DefaultProfileURL is the OAuth profile endpoint used by the post-token
// FetchProfileInfo step.
const DefaultProfileURL = DefaultBaseAPIURL + "/api/oauth/profile"

// DefaultClaudeAISuccessURL is the post-callback redirect target for Claude.ai
// logins (scopes include `user:inference`).
const DefaultClaudeAISuccessURL = "https://platform.claude.com/oauth/code/success?app=claude-code"

// DefaultConsoleSuccessURL is the post-callback redirect target for
// console-only logins.
const DefaultConsoleSuccessURL = "https://platform.claude.com/buy_credits?returnUrl=/oauth/code/success%3Fapp%3Dclaude-code"

// DefaultManualRedirectURL is the redirect_uri used when the user pastes the
// authorization code by hand instead of letting the localhost listener catch
// the redirect.
const DefaultManualRedirectURL = "https://platform.claude.com/oauth/code/callback"

// DefaultClientID is the OAuth client identifier registered for Claude Code.
const DefaultClientID = "9d1c250a-e61b-44d9-88ed-5944d1962f5e"

// OAuthBetaHeader is sent on profile and token-exchange requests to opt into
// the OAuth-aware API surface. Mirrors OAUTH_BETA_HEADER on the TS side.
const OAuthBetaHeader = "oauth-2025-04-20"

// Default OAuth scope strings.
const (
	// ScopeUserInference selects the Claude.ai inference scope and gates
	// the Claude.ai success page.
	ScopeUserInference = "user:inference"
	// ScopeUserProfile is the profile-read scope shared by both flows.
	ScopeUserProfile = "user:profile"
	// ScopeUserSessionsClaudeCode allows Claude Code session APIs.
	ScopeUserSessionsClaudeCode = "user:sessions:claude_code"
	// ScopeUserMcpServers grants access to the user's MCP server list.
	ScopeUserMcpServers = "user:mcp_servers"
	// ScopeUserFileUpload grants the file-upload capability used by tool
	// attachments.
	ScopeUserFileUpload = "user:file_upload"
	// ScopeOrgCreateAPIKey is the console-only scope that allows API key
	// creation on behalf of the signed-in organization.
	ScopeOrgCreateAPIKey = "org:create_api_key"
)

// DefaultClaudeAIScopes is the default scope set for Claude.ai logins.
// Mirrors CLAUDE_AI_OAUTH_SCOPES in src/constants/oauth.ts.
var DefaultClaudeAIScopes = []string{
	ScopeUserProfile,
	ScopeUserInference,
	ScopeUserSessionsClaudeCode,
	ScopeUserMcpServers,
	ScopeUserFileUpload,
}

// DefaultConsoleScopes is the default scope set for console-only logins.
// Mirrors CONSOLE_OAUTH_SCOPES in src/constants/oauth.ts.
var DefaultConsoleScopes = []string{
	ScopeOrgCreateAPIKey,
	ScopeUserProfile,
}

// DefaultAllScopes is the union of Claude.ai and console scopes (with order
// preserved and duplicates removed). This matches ALL_OAUTH_SCOPES on the TS
// side and is the default scope set requested by /login when the caller does
// not specify InferenceOnly.
var DefaultAllScopes = []string{
	ScopeOrgCreateAPIKey,
	ScopeUserProfile,
	ScopeUserInference,
	ScopeUserSessionsClaudeCode,
	ScopeUserMcpServers,
	ScopeUserFileUpload,
}
