package settingssync

import (
	"sync"

	"github.com/sheepzhao/claude-code-go/internal/core/config"
	"github.com/sheepzhao/claude-code-go/internal/platform/oauth"
)

// SubscriptionLoader is the contract used to obtain the currently-authenticated
// Claude.ai OAuth tokens. Mirrors the policylimits package pattern so tests can
// supply a fake without spinning up the full OAuth service stack.
type SubscriptionLoader interface {
	// LoadOAuthTokens returns the current tokens or (nil, nil) when no
	// subscriber is logged in. Returning a non-nil error signals a real load
	// failure that callers may want to surface.
	LoadOAuthTokens() (*oauth.OAuthTokens, error)
}

// SubscriptionLoaderFunc adapts a plain function into the SubscriptionLoader
// contract. Useful for bootstrap wiring.
type SubscriptionLoaderFunc func() (*oauth.OAuthTokens, error)

// LoadOAuthTokens implements SubscriptionLoader.
func (f SubscriptionLoaderFunc) LoadOAuthTokens() (*oauth.OAuthTokens, error) {
	if f == nil {
		return nil, nil
	}
	return f()
}

var (
	subscriptionLoaderMu sync.RWMutex
	subscriptionLoader   SubscriptionLoader
)

// SetSubscriptionLoader registers the OAuth loader used by gating helpers.
// Safe to call repeatedly; the most recent loader wins.
func SetSubscriptionLoader(loader SubscriptionLoader) {
	subscriptionLoaderMu.Lock()
	defer subscriptionLoaderMu.Unlock()
	subscriptionLoader = loader
}

func loadCurrentTokens() (*oauth.OAuthTokens, error) {
	subscriptionLoaderMu.RLock()
	loader := subscriptionLoader
	subscriptionLoaderMu.RUnlock()
	if loader == nil {
		return nil, nil
	}
	return loader.LoadOAuthTokens()
}

var (
	configMu sync.RWMutex
	cfg      *config.Config
)

// SetConfig registers the runtime config used by eligibility and sync helpers.
// Safe to call repeatedly; the most recent config wins.
func SetConfig(c *config.Config) {
	configMu.Lock()
	defer configMu.Unlock()
	cfg = c
}

func getConfig() *config.Config {
	configMu.RLock()
	defer configMu.RUnlock()
	return cfg
}

// IsUsingOAuth reports whether the current user is authenticated with
// first-party OAuth and has the Claude.ai inference scope.
//
// This mirrors the TS isUsingOAuth() function:
//   - provider must be first-party Anthropic (not 3p or custom base URL)
//   - must have a non-empty OAuth access token
//   - scopes must include user:inference
//
// Only first-party Anthropic OAuth is checked — API key users and 3p
// provider users return false, which causes settings sync to silently skip.
func IsUsingOAuth() bool {
	c := getConfig()
	if c == nil {
		return false
	}

	// 3p provider users should not sync.
	if c.Provider != "" && c.Provider != "anthropic" {
		return false
	}

	// Custom base URL users should not sync.
	if c.APIBaseURL != "" && c.APIBaseURL != oauth.DefaultBaseAPIURL {
		return false
	}

	tokens, err := loadCurrentTokens()
	if err != nil || tokens == nil || tokens.AccessToken == "" {
		return false
	}

	// Must have Claude.ai inference scope.
	for _, s := range tokens.Scopes {
		if s == oauth.ScopeUserInference {
			return true
		}
	}
	return false
}

// AccessToken returns the current OAuth access token for HTTP requests.
// Returns empty string when no token is available.
func AccessToken() string {
	tokens, err := loadCurrentTokens()
	if err != nil || tokens == nil {
		return ""
	}
	return tokens.AccessToken
}
