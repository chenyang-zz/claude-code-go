package commands

import (
	"context"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
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
