package policylimits

import (
	"sync"

	"github.com/sheepzhao/claude-code-go/internal/core/config"
	"github.com/sheepzhao/claude-code-go/internal/platform/oauth"
)

// SubscriptionLoader is the contract used to obtain the currently-authenticated
// Claude.ai OAuth tokens. The contract is small so tests can supply a fake
// without spinning up the full OAuth service stack.
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

// SetSubscriptionLoader registers the OAuth loader used by the gating helpers.
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

// SetConfig registers the runtime config used by eligibility checks.
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

// IsEligible reports whether the current user is eligible for policy limits.
//
// Eligibility rules (mirroring the TS side):
//   - 3p provider users are NOT eligible.
//   - Custom base URL users are NOT eligible.
//   - Console users (API key) are eligible if an API key is configured.
//   - OAuth users are eligible only when they have Claude.ai inference scope
//     AND their subscription type is team or enterprise.
func IsEligible() bool {
	c := getConfig()
	if c == nil {
		return false
	}

	// 3p provider users should not hit the policy limits endpoint.
	if c.Provider != "" && c.Provider != "anthropic" {
		return false
	}

	// Custom base URL users should not hit the policy limits endpoint.
	if c.APIBaseURL != "" && c.APIBaseURL != oauth.DefaultBaseAPIURL {
		return false
	}

	// Console users (API key) are eligible if we can get the actual key.
	if c.APIKey != "" {
		return true
	}

	// For OAuth users, check if they have Claude.ai tokens.
	tokens, err := loadCurrentTokens()
	if err != nil || tokens == nil || tokens.AccessToken == "" {
		return false
	}

	// Must have Claude.ai inference scope.
	hasInferenceScope := false
	for _, s := range tokens.Scopes {
		if s == oauth.ScopeUserInference {
			hasInferenceScope = true
			break
		}
	}
	if !hasInferenceScope {
		return false
	}

	// Only Team and Enterprise OAuth users are eligible.
	switch tokens.SubscriptionType {
	case oauth.SubscriptionTeam, oauth.SubscriptionEnterprise:
		return true
	default:
		return false
	}
}
