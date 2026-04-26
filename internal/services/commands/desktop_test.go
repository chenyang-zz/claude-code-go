package commands

import (
	"context"
	"reflect"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestDesktopCommandMetadata verifies /desktop is exposed with the expected canonical descriptor.
func TestDesktopCommandMetadata(t *testing.T) {
	meta := DesktopCommand{}.Metadata()

	if !reflect.DeepEqual(meta, command.Metadata{
		Name:        "desktop",
		Aliases:     []string{"app"},
		Description: "Continue the current session in Claude Desktop",
		Usage:       "/desktop",
	}) {
		t.Fatalf("Metadata() = %#v, want desktop metadata", meta)
	}
}

// TestDesktopCommandExecute verifies /desktop returns the stable fallback.
func TestDesktopCommandExecute(t *testing.T) {
	result, err := DesktopCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != desktopCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, desktopCommandFallback)
	}
}
