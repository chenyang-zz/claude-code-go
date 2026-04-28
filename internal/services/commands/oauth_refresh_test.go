package commands

import (
	"context"
	"reflect"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestOAuthRefreshCommandMetadata verifies /oauth-refresh is exposed as a hidden command.
func TestOAuthRefreshCommandMetadata(t *testing.T) {
	meta := OAuthRefreshCommand{}.Metadata()

	if !reflect.DeepEqual(meta, command.Metadata{
		Name:        "oauth-refresh",
		Description: "Refresh OAuth credentials",
		Usage:       "/oauth-refresh",
		Hidden:      true,
	}) {
		t.Fatalf("Metadata() = %#v, want oauth-refresh metadata", meta)
	}
}

// TestOAuthRefreshCommandExecute verifies /oauth-refresh returns the stable fallback for no-arg execution.
func TestOAuthRefreshCommandExecute(t *testing.T) {
	result, err := OAuthRefreshCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != oauthRefreshCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, oauthRefreshCommandFallback)
	}
}

// TestOAuthRefreshCommandExecuteRejectsArgs verifies /oauth-refresh accepts no arguments.
func TestOAuthRefreshCommandExecuteRejectsArgs(t *testing.T) {
	_, err := OAuthRefreshCommand{}.Execute(context.Background(), command.Args{RawLine: "now"})
	if err == nil {
		t.Fatal("Execute() error = nil, want usage error")
	}
	if err.Error() != "usage: /oauth-refresh" {
		t.Fatalf("Execute() error = %q, want usage error", err.Error())
	}
}
