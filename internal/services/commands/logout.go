package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
	"github.com/sheepzhao/claude-code-go/internal/platform/config"
	"github.com/sheepzhao/claude-code-go/internal/platform/oauth"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// LogoutCommand renders the /logout slash command. When CredentialStore is
// non-nil the command performs real credential cleanup; otherwise it falls back
// to the stable placeholder text.
type LogoutCommand struct {
	// Config carries the already-resolved runtime configuration snapshot.
	Config coreconfig.Config
	// CredentialStore, when non-nil, deletes the on-disk OAuth credentials.
	CredentialStore *oauth.OAuthCredentialStore
	// SettingsWriter, when non-nil, removes oauthAccount metadata from settings.
	SettingsWriter *config.SettingsWriter
}

// Metadata returns the canonical slash descriptor for /logout.
func (c LogoutCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "logout",
		Description: "Sign out from your Anthropic account",
		Usage:       "/logout",
	}
}

// Execute reports the stable logout guidance supported by the current Go host.
// When CredentialStore is available it performs real credential deletion and
// settings cleanup; otherwise it falls back to the pre-migration placeholder.
func (c LogoutCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = args

	usingAPIKey := strings.TrimSpace(c.Config.APIKey) != ""
	usingAuthToken := strings.TrimSpace(c.Config.AuthToken) != ""
	logger.DebugCF("commands", "rendered logout command output", map[string]any{
		"using_api_key":     usingAPIKey,
		"using_auth_token":  usingAuthToken,
		"has_credential_store": c.CredentialStore != nil,
	})

	if usingAPIKey {
		return command.Result{
			Output: "Claude Code Go is using configured API key authentication. Remove the API key from settings or environment variables to sign out.",
		}, nil
	}
	if usingAuthToken {
		return command.Result{
			Output: "Claude Code Go is using configured auth token authentication. Remove the auth token from settings or environment variables to sign out.",
		}, nil
	}

	if c.CredentialStore == nil {
		return command.Result{
			Output: "Interactive Anthropic account logout is not supported in Claude Code Go yet because account login is not available.",
		}, nil
	}

	if err := c.CredentialStore.Delete(); err != nil {
		return command.Result{}, fmt.Errorf("/logout: failed to clear credentials: %w", err)
	}

	if c.SettingsWriter != nil {
		if err := c.SettingsWriter.Unset(ctx, "user", "oauthAccount"); err != nil {
			logger.WarnCF("commands", "failed to clear oauthAccount settings", map[string]any{
				"error": err.Error(),
			})
		}
	}

	return command.Result{
		Output: "Successfully logged out from your Anthropic account.",
	}, nil
}
