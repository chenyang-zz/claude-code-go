package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// profileFetchTimeout matches the 10-second timeout used by the TypeScript
// reference (`src/services/oauth/getOauthProfile.ts`).
const profileFetchTimeout = 10 * time.Second

// FetchProfileInfoOptions captures the inputs to FetchProfileInfo.
type FetchProfileInfoOptions struct {
	// AccessToken is the OAuth access token to be passed in the
	// `Authorization: Bearer ...` header.
	AccessToken string
	// ProfileURL overrides DefaultProfileURL.
	ProfileURL string
	// HTTPClient, when non-nil, is used to issue the GET. When nil a fresh
	// http.Client with Timeout=10s is used.
	HTTPClient *http.Client
}

// FetchProfileInfo retrieves the OAuth profile envelope for the supplied
// access token and projects it into the normalized ProfileInfo shape used by
// callers. Returns an error on any non-2xx status.
func FetchProfileInfo(ctx context.Context, opts FetchProfileInfoOptions) (*ProfileInfo, error) {
	if opts.AccessToken == "" {
		return nil, fmt.Errorf("oauth: FetchProfileInfo: access token is empty")
	}
	profileURL := opts.ProfileURL
	if profileURL == "" {
		profileURL = DefaultProfileURL
	}
	httpClient := opts.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: profileFetchTimeout}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, profileURL, nil)
	if err != nil {
		return nil, fmt.Errorf("oauth: build profile request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+opts.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("anthropic-beta", OAuthBetaHeader)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("oauth: profile fetch GET: %w", err)
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("oauth: read profile response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("oauth: profile fetch failed (status %d %s)", resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	var raw OAuthProfileResponse
	if err := json.Unmarshal(rawBody, &raw); err != nil {
		return nil, fmt.Errorf("oauth: decode profile response: %w", err)
	}

	info := &ProfileInfo{
		SubscriptionType:      mapSubscriptionType(raw.Organization.OrganizationType),
		RateLimitTier:         RateLimitTier(raw.Organization.RateLimitTier),
		DisplayName:           raw.Account.DisplayName,
		HasExtraUsageEnabled:  raw.Organization.HasExtraUsageEnabled,
		BillingType:           BillingType(raw.Organization.BillingType),
		AccountCreatedAt:      raw.Account.CreatedAt,
		SubscriptionCreatedAt: raw.Organization.SubscriptionCreatedAt,
		RawProfile:            raw,
	}
	logger.DebugCF("oauth", "profile fetched", map[string]any{
		"account_uuid":        raw.Account.UUID,
		"organization_uuid":   raw.Organization.UUID,
		"subscription_type":   string(info.SubscriptionType),
		"rate_limit_tier":     string(info.RateLimitTier),
		"has_display_name":    info.DisplayName != "",
		"has_extra_usage":     info.HasExtraUsageEnabled,
	})
	return info, nil
}

// mapSubscriptionType projects the raw `organization_type` string returned by
// the profile endpoint onto the canonical SubscriptionType set. Unknown
// values produce an empty SubscriptionType (TS reference returns null).
func mapSubscriptionType(orgType string) SubscriptionType {
	switch orgType {
	case "claude_max":
		return SubscriptionMax
	case "claude_pro":
		return SubscriptionPro
	case "claude_enterprise":
		return SubscriptionEnterprise
	case "claude_team":
		return SubscriptionTeam
	default:
		return ""
	}
}
