package commands

import (
	"context"
	"fmt"
	"strings"
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

// TestLoginCommandExecute_WithOAuthRunner_Success verifies that when a Login
// runner is supplied and no API key/auth token is set, the command runs the
// runner and formats the resulting LoginOutcome as the slash command output.
func TestLoginCommandExecute_WithOAuthRunner_Success(t *testing.T) {
	cmd := LoginCommand{
		Login: func(ctx context.Context) (*LoginOutcome, error) {
			return &LoginOutcome{
				Email:            "user@example.com",
				OrganizationName: "Acme",
				OrganizationUUID: "org-1",
				SubscriptionType: "max",
				ScopeCount:       6,
				CredentialsPath:  "/home/u/.claude/.credentials.json",
			}, nil
		},
	}
	result, err := cmd.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.HasPrefix(result.Output, "Logged in to your Anthropic account.") {
		t.Fatalf("expected success header, got %q", result.Output)
	}
	for _, want := range []string{
		"Email: user@example.com",
		"Organization: Acme",
		"Subscription: max",
		"Scopes granted: 6",
		"Credentials saved to: /home/u/.claude/.credentials.json",
	} {
		if !strings.Contains(result.Output, want) {
			t.Fatalf("expected %q in output, got:\n%s", want, result.Output)
		}
	}
}

// TestLoginCommandExecute_WithOAuthRunner_OrganizationFallback verifies that
// the organization UUID is shown when no organization name is available.
func TestLoginCommandExecute_WithOAuthRunner_OrganizationFallback(t *testing.T) {
	cmd := LoginCommand{
		Login: func(ctx context.Context) (*LoginOutcome, error) {
			return &LoginOutcome{
				Email:            "u@e.com",
				OrganizationUUID: "org-uuid-only",
				ScopeCount:       1,
				CredentialsPath:  "/path",
			}, nil
		},
	}
	result, _ := cmd.Execute(context.Background(), command.Args{})
	if !strings.Contains(result.Output, "Organization: org-uuid-only") {
		t.Fatalf("expected UUID fallback, got:\n%s", result.Output)
	}
}

// TestLoginCommandExecute_WithOAuthRunner_Skipped verifies that a skipped
// persistence decision is surfaced as a SkipReason instead of a credentials
// path.
func TestLoginCommandExecute_WithOAuthRunner_Skipped(t *testing.T) {
	cmd := LoginCommand{
		Login: func(ctx context.Context) (*LoginOutcome, error) {
			return &LoginOutcome{
				Email:      "u@e.com",
				ScopeCount: 1,
				Skipped:    true,
				SkipReason: "scope set does not include user:inference",
			}, nil
		},
	}
	result, _ := cmd.Execute(context.Background(), command.Args{})
	if !strings.Contains(result.Output, "Credentials NOT persisted: scope set does not include user:inference") {
		t.Fatalf("expected skip reason in output, got:\n%s", result.Output)
	}
	if strings.Contains(result.Output, "Credentials saved to:") {
		t.Fatalf("expected no credentials-saved line when Skipped=true, got:\n%s", result.Output)
	}
}

// TestLoginCommandExecute_WithOAuthRunner_Error verifies that runner errors
// are surfaced as command errors.
func TestLoginCommandExecute_WithOAuthRunner_Error(t *testing.T) {
	wantErr := fmt.Errorf("upstream is angry")
	cmd := LoginCommand{
		Login: func(ctx context.Context) (*LoginOutcome, error) {
			return nil, wantErr
		},
	}
	_, err := cmd.Execute(context.Background(), command.Args{})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "upstream is angry") {
		t.Fatalf("expected wrapped runner error, got %v", err)
	}
}

// TestLoginCommandExecute_WithOAuthRunner_NilOutcome verifies that a runner
// returning nil outcome with no error yields a defensive command error.
func TestLoginCommandExecute_WithOAuthRunner_NilOutcome(t *testing.T) {
	cmd := LoginCommand{
		Login: func(ctx context.Context) (*LoginOutcome, error) {
			return nil, nil
		},
	}
	_, err := cmd.Execute(context.Background(), command.Args{})
	if err == nil {
		t.Fatalf("expected nil-outcome error, got nil")
	}
}

// TestLoginCommandExecute_RunnerSkippedWhenAPIKeyConfigured verifies that the
// API key takes priority over the OAuth runner: even with a non-nil Login,
// configured API keys keep the legacy fallback text.
func TestLoginCommandExecute_RunnerSkippedWhenAPIKeyConfigured(t *testing.T) {
	called := 0
	cmd := LoginCommand{
		Config: coreconfig.Config{APIKey: "ak"},
		Login: func(ctx context.Context) (*LoginOutcome, error) {
			called++
			return nil, nil
		},
	}
	result, err := cmd.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if called != 0 {
		t.Fatalf("Login runner should not be called when API key is configured")
	}
	if !strings.Contains(result.Output, "configured API key authentication") {
		t.Fatalf("expected API-key fallback text, got %q", result.Output)
	}
}
