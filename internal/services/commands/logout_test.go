package commands

import (
	"context"
	"strings"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
	"github.com/sheepzhao/claude-code-go/internal/platform/oauth"
)

// TestLogoutCommandMetadata verifies /logout exposes stable metadata.
func TestLogoutCommandMetadata(t *testing.T) {
	meta := LogoutCommand{}.Metadata()
	if meta.Name != "logout" {
		t.Fatalf("Metadata().Name = %q, want logout", meta.Name)
	}
	if meta.Description != "Sign out from your Anthropic account" {
		t.Fatalf("Metadata().Description = %q, want stable logout description", meta.Description)
	}
	if meta.Usage != "/logout" {
		t.Fatalf("Metadata().Usage = %q, want /logout", meta.Usage)
	}
}

// TestLogoutCommandExecuteWithoutAPIKey verifies /logout reports the current Go host fallback when interactive auth is unavailable.
func TestLogoutCommandExecuteWithoutAPIKey(t *testing.T) {
	result, err := LogoutCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	want := "Interactive Anthropic account logout is not supported in Claude Code Go yet because account login is not available."
	if result.Output != want {
		t.Fatalf("Execute() output = %q, want %q", result.Output, want)
	}
}

// TestLogoutCommandExecuteWithAuthToken verifies /logout acknowledges configured auth token authentication has no interactive logout flow.
func TestLogoutCommandExecuteWithAuthToken(t *testing.T) {
	result, err := LogoutCommand{
		Config: coreconfig.Config{
			AuthToken: "auth-token",
		},
	}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	want := "Claude Code Go is using configured auth token authentication. Remove the auth token from settings or environment variables to sign out."
	if result.Output != want {
		t.Fatalf("Execute() output = %q, want %q", result.Output, want)
	}
}

// TestLogoutCommandExecuteWithAPIKey verifies /logout acknowledges configuration-based authentication has no interactive logout flow.
func TestLogoutCommandExecuteWithAPIKey(t *testing.T) {
	result, err := LogoutCommand{
		Config: coreconfig.Config{
			APIKey: "test-key",
		},
	}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	want := "Claude Code Go is using configured API key authentication. Remove the API key from settings or environment variables to sign out."
	if result.Output != want {
		t.Fatalf("Execute() output = %q, want %q", result.Output, want)
	}
}

// TestLogoutCommandExecute_RealLogout verifies /logout deletes credentials when wired.
func TestLogoutCommandExecute_RealLogout(t *testing.T) {
	dir := t.TempDir()
	store, err := oauth.NewOAuthCredentialStore(dir)
	if err != nil {
		t.Fatalf("NewOAuthCredentialStore: %v", err)
	}

	// Seed credentials
	tokens := &oauth.OAuthTokens{
		AccessToken:  "at",
		RefreshToken: "rt",
		ExpiresAt:    1,
		Scopes:       []string{oauth.ScopeUserInference},
	}
	if _, err := store.Save(tokens); err != nil {
		t.Fatalf("Save: %v", err)
	}

	cmd := LogoutCommand{
		Config:          coreconfig.Config{HomeDir: dir},
		CredentialStore: store,
	}
	ctx := context.Background()
	result, err := cmd.Execute(ctx, command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	want := "Successfully logged out from your Anthropic account."
	if result.Output != want {
		t.Fatalf("Execute() output = %q, want %q", result.Output, want)
	}

	// Verify credentials deleted
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load after logout: %v", err)
	}
	if loaded != nil {
		t.Fatalf("expected credentials deleted, got %+v", loaded)
	}
}

// TestLogoutCommandExecute_RealLogout_NoSettingsWriter verifies /logout still
// deletes credentials when SettingsWriter is unavailable.
func TestLogoutCommandExecute_RealLogout_NoSettingsWriter(t *testing.T) {
	dir := t.TempDir()
	store, err := oauth.NewOAuthCredentialStore(dir)
	if err != nil {
		t.Fatalf("NewOAuthCredentialStore: %v", err)
	}
	tokens := &oauth.OAuthTokens{
		AccessToken:  "at",
		RefreshToken: "rt",
		ExpiresAt:    1,
		Scopes:       []string{oauth.ScopeUserInference},
	}
	if _, err := store.Save(tokens); err != nil {
		t.Fatalf("Save: %v", err)
	}

	cmd := LogoutCommand{
		Config:          coreconfig.Config{HomeDir: dir},
		CredentialStore: store,
	}
	result, err := cmd.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(result.Output, "Successfully logged out") {
		t.Fatalf("Execute() output = %q", result.Output)
	}
}

// TestLogoutCommandExecute_BackwardCompatibility verifies /logout without
// CredentialStore still returns the placeholder text.
func TestLogoutCommandExecute_BackwardCompatibility(t *testing.T) {
	result, err := LogoutCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	want := "Interactive Anthropic account logout is not supported in Claude Code Go yet because account login is not available."
	if result.Output != want {
		t.Fatalf("Execute() output = %q, want %q", result.Output, want)
	}
}
