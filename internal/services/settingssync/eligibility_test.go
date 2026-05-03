package settingssync

import (
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/config"
	"github.com/sheepzhao/claude-code-go/internal/platform/oauth"
)

func resetEligibility() {
	SetConfig(nil)
	SetSubscriptionLoader(nil)
}

func TestIsUsingOAuth_3pProvider(t *testing.T) {
	resetEligibility()
	SetConfig(&config.Config{Provider: "openai"})
	if IsUsingOAuth() {
		t.Error("3p provider should not be using OAuth")
	}
}

func TestIsUsingOAuth_CustomBaseURL(t *testing.T) {
	resetEligibility()
	SetConfig(&config.Config{
		Provider:   "anthropic",
		APIBaseURL: "https://custom.example.com",
	})
	if IsUsingOAuth() {
		t.Error("custom base URL should not be OAuth")
	}
}

func TestIsUsingOAuth_NoConfig(t *testing.T) {
	resetEligibility()
	if IsUsingOAuth() {
		t.Error("nil config should return false")
	}
}

func TestIsUsingOAuth_NoTokens(t *testing.T) {
	resetEligibility()
	SetConfig(&config.Config{Provider: "anthropic"})
	SetSubscriptionLoader(SubscriptionLoaderFunc(func() (*oauth.OAuthTokens, error) {
		return nil, nil
	}))
	if IsUsingOAuth() {
		t.Error("no tokens should return false")
	}
}

func TestIsUsingOAuth_NoAccessToken(t *testing.T) {
	resetEligibility()
	SetConfig(&config.Config{Provider: "anthropic"})
	SetSubscriptionLoader(SubscriptionLoaderFunc(func() (*oauth.OAuthTokens, error) {
		return &oauth.OAuthTokens{
			AccessToken: "",
			Scopes:      []string{oauth.ScopeUserInference},
		}, nil
	}))
	if IsUsingOAuth() {
		t.Error("empty access token should return false")
	}
}

func TestIsUsingOAuth_MissingInferenceScope(t *testing.T) {
	resetEligibility()
	SetConfig(&config.Config{Provider: "anthropic"})
	SetSubscriptionLoader(SubscriptionLoaderFunc(func() (*oauth.OAuthTokens, error) {
		return &oauth.OAuthTokens{
			AccessToken: "token-abc",
			Scopes:      []string{"user:profile"},
		}, nil
	}))
	if IsUsingOAuth() {
		t.Error("missing inference scope should return false")
	}
}

func TestIsUsingOAuth_HasInferenceScope(t *testing.T) {
	resetEligibility()
	SetConfig(&config.Config{Provider: "anthropic"})
	SetSubscriptionLoader(SubscriptionLoaderFunc(func() (*oauth.OAuthTokens, error) {
		return &oauth.OAuthTokens{
			AccessToken: "token-abc",
			Scopes:      []string{oauth.ScopeUserInference},
		}, nil
	}))
	if !IsUsingOAuth() {
		t.Error("valid OAuth with inference scope should return true")
	}
}

func TestIsUsingOAuth_DefaultProvider(t *testing.T) {
	resetEligibility()
	SetConfig(&config.Config{Provider: ""})
	SetSubscriptionLoader(SubscriptionLoaderFunc(func() (*oauth.OAuthTokens, error) {
		return &oauth.OAuthTokens{
			AccessToken: "token-abc",
			Scopes:      []string{oauth.ScopeUserInference},
		}, nil
	}))
	if !IsUsingOAuth() {
		t.Error("default (empty) provider should be treated as first-party")
	}
}

func TestAccessToken_HasToken(t *testing.T) {
	resetEligibility()
	SetSubscriptionLoader(SubscriptionLoaderFunc(func() (*oauth.OAuthTokens, error) {
		return &oauth.OAuthTokens{
			AccessToken: "secret-token",
		}, nil
	}))
	if got := AccessToken(); got != "secret-token" {
		t.Errorf("AccessToken: got %q, want %q", got, "secret-token")
	}
}

func TestAccessToken_NoTokens(t *testing.T) {
	resetEligibility()
	SetSubscriptionLoader(SubscriptionLoaderFunc(func() (*oauth.OAuthTokens, error) {
		return nil, nil
	}))
	if got := AccessToken(); got != "" {
		t.Errorf("AccessToken with no tokens: got %q, want empty", got)
	}
}

func TestSubscriptionLoaderFunc_Nil(t *testing.T) {
	var f SubscriptionLoaderFunc
	tokens, err := f.LoadOAuthTokens()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if tokens != nil {
		t.Error("nil func should return nil tokens")
	}
}
