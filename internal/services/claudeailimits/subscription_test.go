package claudeailimits

import (
	"errors"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/platform/oauth"
)

// fakeSubscriptionLoader implements SubscriptionLoader with deterministic
// behaviour for tests.
type fakeSubscriptionLoader struct {
	tokens *oauth.OAuthTokens
	err    error
}

func (f *fakeSubscriptionLoader) LoadOAuthTokens() (*oauth.OAuthTokens, error) {
	return f.tokens, f.err
}

// withSubscription installs a loader for the duration of one test.
func withSubscription(t *testing.T, loader SubscriptionLoader) {
	t.Helper()
	SetSubscriptionLoader(loader)
	t.Cleanup(func() { SetSubscriptionLoader(nil) })
}

func TestIsClaudeAISubscriberReturnsFalseWithoutLoader(t *testing.T) {
	SetSubscriptionLoader(nil)
	if IsClaudeAISubscriber() {
		t.Fatal("expected false when no loader registered")
	}
}

func TestIsClaudeAISubscriberHonoursLoader(t *testing.T) {
	withSubscription(t, &fakeSubscriptionLoader{
		tokens: &oauth.OAuthTokens{SubscriptionType: oauth.SubscriptionMax},
	})
	if !IsClaudeAISubscriber() {
		t.Fatal("expected true when subscription is set")
	}

	withSubscription(t, &fakeSubscriptionLoader{tokens: &oauth.OAuthTokens{}})
	if IsClaudeAISubscriber() {
		t.Fatal("expected false when SubscriptionType is empty")
	}
}

func TestIsClaudeAISubscriberSwallowsLoaderError(t *testing.T) {
	withSubscription(t, &fakeSubscriptionLoader{err: errors.New("boom")})
	if IsClaudeAISubscriber() {
		t.Fatal("expected false when loader returns error")
	}
}

func TestGetSubscriptionType(t *testing.T) {
	withSubscription(t, &fakeSubscriptionLoader{
		tokens: &oauth.OAuthTokens{SubscriptionType: oauth.SubscriptionPro},
	})
	if got := GetSubscriptionType(); got != oauth.SubscriptionPro {
		t.Fatalf("GetSubscriptionType = %q, want pro", got)
	}
}

func TestHasExtraUsageEnabled(t *testing.T) {
	withSubscription(t, &fakeSubscriptionLoader{
		tokens: &oauth.OAuthTokens{
			SubscriptionType:     oauth.SubscriptionTeam,
			HasExtraUsageEnabled: true,
		},
	})
	if !HasExtraUsageEnabled() {
		t.Fatal("expected true when bit is set")
	}

	withSubscription(t, &fakeSubscriptionLoader{
		tokens: &oauth.OAuthTokens{SubscriptionType: oauth.SubscriptionTeam},
	})
	if HasExtraUsageEnabled() {
		t.Fatal("expected false when bit is unset")
	}
}

func TestHasClaudeAIBillingAccess(t *testing.T) {
	cases := []struct {
		tier oauth.SubscriptionType
		want bool
	}{
		{tier: oauth.SubscriptionPro, want: true},
		{tier: oauth.SubscriptionMax, want: true},
		{tier: oauth.SubscriptionTeam, want: false},
		{tier: oauth.SubscriptionEnterprise, want: false},
		{tier: "", want: false},
	}
	for _, tc := range cases {
		withSubscription(t, &fakeSubscriptionLoader{
			tokens: &oauth.OAuthTokens{SubscriptionType: tc.tier},
		})
		if got := HasClaudeAIBillingAccess(); got != tc.want {
			t.Fatalf("HasClaudeAIBillingAccess() for %q = %v, want %v", tc.tier, got, tc.want)
		}
	}
}

func TestIsOverageProvisioningAllowed(t *testing.T) {
	cases := []struct {
		name        string
		tier        oauth.SubscriptionType
		billingType oauth.BillingType
		want        bool
	}{
		{name: "team_no_billing", tier: oauth.SubscriptionTeam, want: true},
		{name: "team_aws_marketplace", tier: oauth.SubscriptionTeam, billingType: "aws_marketplace", want: false},
		{name: "team_anthropic_api", tier: oauth.SubscriptionTeam, billingType: "anthropic_api", want: true},
		{name: "enterprise_aws", tier: oauth.SubscriptionEnterprise, billingType: "aws_marketplace", want: false},
		{name: "pro_blocked", tier: oauth.SubscriptionPro, want: false},
		{name: "max_blocked", tier: oauth.SubscriptionMax, want: false},
		{name: "no_subscription", tier: "", want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withSubscription(t, &fakeSubscriptionLoader{
				tokens: &oauth.OAuthTokens{
					SubscriptionType: tc.tier,
					BillingType:      tc.billingType,
				},
			})
			if got := IsOverageProvisioningAllowed(); got != tc.want {
				t.Fatalf("IsOverageProvisioningAllowed = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestSubscriptionLoaderFunc(t *testing.T) {
	called := false
	loader := SubscriptionLoaderFunc(func() (*oauth.OAuthTokens, error) {
		called = true
		return &oauth.OAuthTokens{SubscriptionType: oauth.SubscriptionMax}, nil
	})
	withSubscription(t, loader)
	if !IsClaudeAISubscriber() {
		t.Fatal("loader func should be honoured")
	}
	if !called {
		t.Fatal("loader func should be called")
	}
}
