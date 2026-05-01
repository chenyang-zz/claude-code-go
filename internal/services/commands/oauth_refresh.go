package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/internal/platform/config"
	"github.com/sheepzhao/claude-code-go/internal/platform/oauth"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const oauthRefreshCommandFallback = "OAuth refresh command is not available in Claude Code Go yet. Refresh token exchange and account sync flows remain unmigrated."

// OAuthRefreshCommand exposes the /oauth-refresh slash command. When
// CredentialStore is non-nil the command performs a real refresh-token
// exchange; otherwise it falls back to the stable placeholder text.
type OAuthRefreshCommand struct {
	// CredentialStore, when non-nil, loads existing and persists refreshed tokens.
	CredentialStore *oauth.OAuthCredentialStore
	// SettingsWriter, when non-nil, updates oauthAccount metadata after refresh.
	SettingsWriter *config.SettingsWriter
}

// Metadata returns the canonical slash descriptor for /oauth-refresh.
func (c OAuthRefreshCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "oauth-refresh",
		Description: "Refresh OAuth credentials",
		Usage:       "/oauth-refresh",
		Hidden:      true,
	}
}

// Execute accepts no arguments and performs a refresh-token exchange when
// CredentialStore is available. Otherwise it reports the stable fallback.
func (c OAuthRefreshCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	raw := strings.TrimSpace(args.RawLine)
	if raw != "" {
		return command.Result{}, fmt.Errorf("usage: %s", c.Metadata().Usage)
	}

	if c.CredentialStore == nil {
		logger.DebugCF("commands", "rendered oauth-refresh command fallback output", map[string]any{
			"oauth_refresh_available": false,
			"hidden_command":          true,
		})
		return command.Result{
			Output: oauthRefreshCommandFallback,
		}, nil
	}

	existing, err := c.CredentialStore.Load()
	if err != nil {
		return command.Result{}, fmt.Errorf("/oauth-refresh: failed to load credentials: %w", err)
	}
	if existing == nil || existing.RefreshToken == "" {
		return command.Result{}, fmt.Errorf("/oauth-refresh: no stored OAuth credentials found")
	}

	tokenResp, err := oauth.RefreshToken(ctx, oauth.RefreshTokenOptions{
		RefreshToken: existing.RefreshToken,
	})
	if err != nil {
		return command.Result{}, fmt.Errorf("/oauth-refresh: token refresh failed: %w", err)
	}

	tokens := &oauth.OAuthTokens{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresAt:    time.Now().UnixMilli() + int64(tokenResp.ExpiresIn)*1000,
		Scopes:       oauth.ParseScopes(tokenResp.Scope),
	}

	decision, saveErr := c.CredentialStore.Save(tokens)
	if saveErr != nil {
		return command.Result{}, fmt.Errorf("/oauth-refresh: failed to persist refreshed credentials: %w", saveErr)
	}
	if !decision.Persisted {
		return command.Result{}, fmt.Errorf("/oauth-refresh: credentials not persisted: %s", decision.SkipReason)
	}

	if c.SettingsWriter != nil && tokenResp.Account != nil {
		if tokenResp.Account.EmailAddress != "" {
			_ = c.SettingsWriter.Set(ctx, "user", "oauthAccount.emailAddress", tokenResp.Account.EmailAddress)
		}
		if tokenResp.Account.UUID != "" {
			_ = c.SettingsWriter.Set(ctx, "user", "oauthAccount.accountUuid", tokenResp.Account.UUID)
		}
	}

	expiresAt := time.UnixMilli(tokens.ExpiresAt).Format(time.RFC3339)
	return command.Result{
		Output: fmt.Sprintf("Successfully refreshed OAuth credentials. Token expires at: %s", expiresAt),
	}, nil
}
