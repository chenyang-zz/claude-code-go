package policylimits

import (
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/config"
	"github.com/sheepzhao/claude-code-go/internal/platform/oauth"
)

func TestIsEligible_3pProvider(t *testing.T) {
	SetConfig(&config.Config{Provider: "openai"})
	if IsEligible() {
		t.Error("3p provider should not be eligible")
	}
}

func TestIsEligible_CustomBaseURL(t *testing.T) {
	SetConfig(&config.Config{
		Provider:   "anthropic",
		APIBaseURL: "https://custom.example.com",
	})
	if IsEligible() {
		t.Error("custom base URL should not be eligible")
	}
}

func TestIsEligible_APIKey(t *testing.T) {
	SetConfig(&config.Config{
		Provider: "anthropic",
		APIKey:   "sk-test",
	})
	if !IsEligible() {
		t.Error("console user with API key should be eligible")
	}
}

func TestIsEligible_OAuthTeam(t *testing.T) {
	SetSubscriptionLoader(SubscriptionLoaderFunc(func() (*oauth.OAuthTokens, error) {
		return &oauth.OAuthTokens{
			AccessToken:      "token",
			Scopes:           []string{oauth.ScopeUserInference},
			SubscriptionType: oauth.SubscriptionTeam,
		}, nil
	}))
	SetConfig(&config.Config{Provider: "anthropic"})
	if !IsEligible() {
		t.Error("OAuth team subscriber should be eligible")
	}
}

func TestIsEligible_OAuthEnterprise(t *testing.T) {
	SetSubscriptionLoader(SubscriptionLoaderFunc(func() (*oauth.OAuthTokens, error) {
		return &oauth.OAuthTokens{
			AccessToken:      "token",
			Scopes:           []string{oauth.ScopeUserInference},
			SubscriptionType: oauth.SubscriptionEnterprise,
		}, nil
	}))
	SetConfig(&config.Config{Provider: "anthropic"})
	if !IsEligible() {
		t.Error("OAuth enterprise subscriber should be eligible")
	}
}

func TestIsEligible_OAuthProNotEligible(t *testing.T) {
	SetSubscriptionLoader(SubscriptionLoaderFunc(func() (*oauth.OAuthTokens, error) {
		return &oauth.OAuthTokens{
			AccessToken:      "token",
			Scopes:           []string{oauth.ScopeUserInference},
			SubscriptionType: oauth.SubscriptionPro,
		}, nil
	}))
	SetConfig(&config.Config{Provider: "anthropic"})
	if IsEligible() {
		t.Error("OAuth pro subscriber should not be eligible")
	}
}

func TestIsEligible_OAuthNoInferenceScope(t *testing.T) {
	SetSubscriptionLoader(SubscriptionLoaderFunc(func() (*oauth.OAuthTokens, error) {
		return &oauth.OAuthTokens{
			AccessToken:      "token",
			Scopes:           []string{oauth.ScopeUserProfile},
			SubscriptionType: oauth.SubscriptionTeam,
		}, nil
	}))
	SetConfig(&config.Config{Provider: "anthropic"})
	if IsEligible() {
		t.Error("OAuth without inference scope should not be eligible")
	}
}

func TestIsEligible_NoAuth(t *testing.T) {
	SetSubscriptionLoader(SubscriptionLoaderFunc(func() (*oauth.OAuthTokens, error) {
		return nil, nil
	}))
	SetConfig(&config.Config{Provider: "anthropic"})
	if IsEligible() {
		t.Error("no auth should not be eligible")
	}
}

func TestIsEligible_NilConfig(t *testing.T) {
	SetConfig(nil)
	if IsEligible() {
		t.Error("nil config should not be eligible")
	}
}
