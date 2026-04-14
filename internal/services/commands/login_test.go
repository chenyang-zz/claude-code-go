package commands

import (
	"context"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
)

// TestLoginCommandMetadata verifies /login exposes stable metadata.
func TestLoginCommandMetadata(t *testing.T) {
	meta := LoginCommand{}.Metadata()
	if meta.Name != "login" {
		t.Fatalf("Metadata().Name = %q, want login", meta.Name)
	}
	if meta.Description != "Sign in with your Anthropic account" {
		t.Fatalf("Metadata().Description = %q, want stable login description", meta.Description)
	}
	if meta.Usage != "/login" {
		t.Fatalf("Metadata().Usage = %q, want /login", meta.Usage)
	}
}

// TestLoginCommandExecuteWithoutAPIKey verifies /login reports the current Go host fallback when interactive auth is unavailable.
func TestLoginCommandExecuteWithoutAPIKey(t *testing.T) {
	result, err := LoginCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	want := "Interactive Anthropic account login is not supported in Claude Code Go yet. Configure an API key or auth token in settings or environment variables instead."
	if result.Output != want {
		t.Fatalf("Execute() output = %q, want %q", result.Output, want)
	}
}

// TestLoginCommandExecuteWithAuthToken verifies /login acknowledges configured auth token authentication.
func TestLoginCommandExecuteWithAuthToken(t *testing.T) {
	result, err := LoginCommand{
		Config: coreconfig.Config{
			AuthToken: "auth-token",
		},
	}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	want := "Claude Code Go is using configured auth token authentication. Interactive account switching is not supported yet."
	if result.Output != want {
		t.Fatalf("Execute() output = %q, want %q", result.Output, want)
	}
}

// TestLoginCommandExecuteWithAPIKey verifies /login acknowledges configured API key authentication.
func TestLoginCommandExecuteWithAPIKey(t *testing.T) {
	result, err := LoginCommand{
		Config: coreconfig.Config{
			APIKey: "test-key",
		},
	}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	want := "Claude Code Go is using configured API key authentication. Interactive account switching is not supported yet."
	if result.Output != want {
		t.Fatalf("Execute() output = %q, want %q", result.Output, want)
	}
}
