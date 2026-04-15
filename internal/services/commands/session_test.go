package commands

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
)

// TestSessionCommandMetadata verifies /session keeps the source-compatible descriptor.
func TestSessionCommandMetadata(t *testing.T) {
	got := SessionCommand{}.Metadata()
	want := command.Metadata{
		Name:        "session",
		Description: "Show remote session URL and QR code",
		Usage:       "/session",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Metadata() = %#v, want %#v", got, want)
	}
}

// TestSessionCommandExecuteReportsRemoteFallback verifies the Go host exposes the stable non-remote fallback before remote mode exists.
func TestSessionCommandExecuteReportsRemoteFallback(t *testing.T) {
	result, err := SessionCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	want := "Not in remote mode. Start with `claude --remote` to use this command."
	if result.Output != want {
		t.Fatalf("Execute() output = %q, want %q", result.Output, want)
	}
}

// TestSessionCommandExecuteRendersRemoteSession verifies `/session` surfaces the minimum remote URL and text QR output when remote mode is wired.
func TestSessionCommandExecuteRendersRemoteSession(t *testing.T) {
	result, err := SessionCommand{
		RemoteSession: coreconfig.RemoteSessionConfig{
			Enabled:   true,
			SessionID: "session_test123",
			URL:       "https://claude.ai/code/session_test123?m=0",
		},
	}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !strings.Contains(result.Output, "Remote session\n") {
		t.Fatalf("Execute() output = %q, want remote session heading", result.Output)
	}
	if !strings.Contains(result.Output, "Open in browser: https://claude.ai/code/session_test123?m=0") {
		t.Fatalf("Execute() output = %q, want remote session url", result.Output)
	}
}
