package commands

import (
	"context"
	"reflect"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
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
