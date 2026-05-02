package claudeailimits

import (
	"sync"

	"github.com/sheepzhao/claude-code-go/internal/platform/oauth"
)

// SubscriptionLoader is the contract the package uses to obtain the
// currently-authenticated Claude.ai OAuth tokens. The contract is small on
// purpose so tests can supply a fake without spinning up the full OAuth
// service stack.
type SubscriptionLoader interface {
	// LoadOAuthTokens returns the current tokens or (nil, nil) when no
	// subscriber is logged in. Returning a non-nil error signals a real
	// load failure that callers may want to surface.
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
	// subscriptionLoader is the package-level loader used by gating
	// helpers. nil means subscription gating is unavailable; helpers will
	// fall back to a "non-subscriber" answer in that case.
	subscriptionLoader SubscriptionLoader
)

// SetSubscriptionLoader registers the OAuth loader used by the gating helpers.
// Safe to call repeatedly; the most recent loader wins.
func SetSubscriptionLoader(loader SubscriptionLoader) {
	subscriptionLoaderMu.Lock()
	defer subscriptionLoaderMu.Unlock()
	subscriptionLoader = loader
}

// loadCurrentTokens returns the tokens supplied by the registered loader or
// nil when no loader is configured. Errors from the loader bubble up so the
// caller can decide how to react.
func loadCurrentTokens() (*oauth.OAuthTokens, error) {
	subscriptionLoaderMu.RLock()
	loader := subscriptionLoader
	subscriptionLoaderMu.RUnlock()
	if loader == nil {
		return nil, nil
	}
	return loader.LoadOAuthTokens()
}

// IsClaudeAISubscriber reports whether a Claude.ai subscriber is currently
// authenticated. Returns false on any load failure so callers default to
// the safer "no subscription" branch.
func IsClaudeAISubscriber() bool {
	tokens, err := loadCurrentTokens()
	if err != nil || tokens == nil {
		return false
	}
	return tokens.SubscriptionType != ""
}

// GetSubscriptionType returns the current subscription tier, or an empty
// string when no subscriber is logged in or the type cannot be determined.
func GetSubscriptionType() oauth.SubscriptionType {
	tokens, err := loadCurrentTokens()
	if err != nil || tokens == nil {
		return ""
	}
	return tokens.SubscriptionType
}

// HasExtraUsageEnabled reports whether the current organization has the
// extra-usage tier enabled. Returns false on any load failure.
func HasExtraUsageEnabled() bool {
	tokens, err := loadCurrentTokens()
	if err != nil || tokens == nil {
		return false
	}
	return tokens.HasExtraUsageEnabled
}

// HasClaudeAIBillingAccess approximates the TS-side hasClaudeAiBillingAccess
// helper. The full TS implementation cross-checks a billing-API endpoint;
// here we use the simple, stable proxy that pro / max users always have
// billing access and team / enterprise users do not. Empty subscriptions
// short-circuit to false.
func HasClaudeAIBillingAccess() bool {
	switch GetSubscriptionType() {
	case oauth.SubscriptionPro, oauth.SubscriptionMax:
		return true
	default:
		return false
	}
}

// recognisedOverageBillingTypes is the set of billing channels known to
// support overage provisioning via the Claude.ai backend. The list mirrors
// the channels surfaced by the TS reference and excludes marketplace tiers
// (e.g. AWS Marketplace) where the backend cannot honour /extra-usage even
// for team / enterprise tiers.
var recognisedOverageBillingTypes = map[oauth.BillingType]struct{}{
	"":                {}, // Empty billing type — profile fetch did not return one.
	"anthropic_api":   {},
	"anthropic":       {},
	"console":         {},
	"console_proxy":   {},
	"claudeai_pro":    {},
	"claudeai_team":   {},
	"claudeai_enterprise": {},
	"stripe":          {},
}

// IsOverageProvisioningAllowed reports whether the current organization can
// provision overage. Mirrors the TS-side gate that team / enterprise tiers
// on recognised billing channels (anthropic_api, console_proxy, marketplace
// "claudeai_*") are eligible. Unknown / unrecognised billing types return
// false so callers do not surface "/extra-usage" upsell text on platforms
// that cannot honour the request.
func IsOverageProvisioningAllowed() bool {
	tokens, err := loadCurrentTokens()
	if err != nil || tokens == nil {
		return false
	}
	switch tokens.SubscriptionType {
	case oauth.SubscriptionTeam, oauth.SubscriptionEnterprise:
		// fall through
	default:
		return false
	}
	if _, ok := recognisedOverageBillingTypes[tokens.BillingType]; !ok {
		return false
	}
	return true
}
