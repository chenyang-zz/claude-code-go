package oauth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFetchProfileInfo_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer at-1" {
			t.Errorf("Authorization = %q, want Bearer at-1", got)
		}
		if got := r.Header.Get("anthropic-beta"); got != OAuthBetaHeader {
			t.Errorf("anthropic-beta = %q, want %q", got, OAuthBetaHeader)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"account": {
				"uuid": "acct-1",
				"email": "u@example.com",
				"display_name": "Alice",
				"created_at": "2024-01-01T00:00:00Z"
			},
			"organization": {
				"uuid": "org-1",
				"organization_type": "claude_max",
				"rate_limit_tier": "default_claude_max_5x",
				"has_extra_usage_enabled": true,
				"billing_type": "stripe",
				"subscription_created_at": "2024-02-01T00:00:00Z"
			}
		}`))
	}))
	defer srv.Close()

	info, err := FetchProfileInfo(context.Background(), FetchProfileInfoOptions{
		AccessToken: "at-1",
		ProfileURL:  srv.URL,
	})
	if err != nil {
		t.Fatalf("FetchProfileInfo: %v", err)
	}
	if info.SubscriptionType != SubscriptionMax {
		t.Fatalf("SubscriptionType = %q, want %q", info.SubscriptionType, SubscriptionMax)
	}
	if string(info.RateLimitTier) != "default_claude_max_5x" {
		t.Fatalf("RateLimitTier = %q", info.RateLimitTier)
	}
	if info.DisplayName != "Alice" {
		t.Fatalf("DisplayName = %q", info.DisplayName)
	}
	if !info.HasExtraUsageEnabled {
		t.Fatalf("HasExtraUsageEnabled should be true")
	}
	if string(info.BillingType) != "stripe" {
		t.Fatalf("BillingType = %q", info.BillingType)
	}
	if info.AccountCreatedAt != "2024-01-01T00:00:00Z" {
		t.Fatalf("AccountCreatedAt = %q", info.AccountCreatedAt)
	}
	if info.SubscriptionCreatedAt != "2024-02-01T00:00:00Z" {
		t.Fatalf("SubscriptionCreatedAt = %q", info.SubscriptionCreatedAt)
	}
	if info.RawProfile.Account.UUID != "acct-1" {
		t.Fatalf("RawProfile.Account.UUID = %q", info.RawProfile.Account.UUID)
	}
}

func TestFetchProfileInfo_OrganizationTypeMapping(t *testing.T) {
	tests := map[string]SubscriptionType{
		"claude_max":        SubscriptionMax,
		"claude_pro":        SubscriptionPro,
		"claude_enterprise": SubscriptionEnterprise,
		"claude_team":       SubscriptionTeam,
		"":                  "",
		"unknown_type":      "",
	}
	for orgType, want := range tests {
		t.Run("type-"+orgType, func(t *testing.T) {
			got := mapSubscriptionType(orgType)
			if got != want {
				t.Fatalf("mapSubscriptionType(%q) = %q, want %q", orgType, got, want)
			}
		})
	}
}

func TestFetchProfileInfo_MissingFieldsAreZero(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"account":{"uuid":"acct"},"organization":{"uuid":"org"}}`))
	}))
	defer srv.Close()

	info, err := FetchProfileInfo(context.Background(), FetchProfileInfoOptions{
		AccessToken: "at",
		ProfileURL:  srv.URL,
	})
	if err != nil {
		t.Fatalf("FetchProfileInfo: %v", err)
	}
	if info.SubscriptionType != "" {
		t.Fatalf("SubscriptionType should be empty for missing organization_type, got %q", info.SubscriptionType)
	}
	if info.DisplayName != "" {
		t.Fatalf("DisplayName should be empty, got %q", info.DisplayName)
	}
}

func TestFetchProfileInfo_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	_, err := FetchProfileInfo(context.Background(), FetchProfileInfoOptions{
		AccessToken: "at",
		ProfileURL:  srv.URL,
	})
	if err == nil || !strings.Contains(err.Error(), "profile fetch failed") {
		t.Fatalf("expected profile-fetch-failed error, got %v", err)
	}
}

func TestFetchProfileInfo_EmptyAccessToken(t *testing.T) {
	_, err := FetchProfileInfo(context.Background(), FetchProfileInfoOptions{
		AccessToken: "",
	})
	if err == nil || !strings.Contains(err.Error(), "access token is empty") {
		t.Fatalf("expected access-token-empty error, got %v", err)
	}
}

func TestFetchProfileInfo_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{this is not json}`))
	}))
	defer srv.Close()

	_, err := FetchProfileInfo(context.Background(), FetchProfileInfoOptions{
		AccessToken: "at",
		ProfileURL:  srv.URL,
	})
	if err == nil || !strings.Contains(err.Error(), "decode profile response") {
		t.Fatalf("expected decode error, got %v", err)
	}
}
