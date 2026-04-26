package commands

import (
	"context"
	"reflect"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestWebSetupCommandMetadata verifies /web-setup is exposed with the expected canonical descriptor.
func TestWebSetupCommandMetadata(t *testing.T) {
	meta := WebSetupCommand{}.Metadata()

	if !reflect.DeepEqual(meta, command.Metadata{
		Name:        "web-setup",
		Description: "Setup Claude Code on the web (requires connecting your GitHub account)",
		Usage:       "/web-setup",
	}) {
		t.Fatalf("Metadata() = %#v, want web-setup metadata", meta)
	}
}

// TestWebSetupCommandExecute verifies /web-setup returns the stable fallback for no-arg execution.
func TestWebSetupCommandExecute(t *testing.T) {
	result, err := WebSetupCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != webSetupCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, webSetupCommandFallback)
	}
}

// TestWebSetupCommandExecuteAcceptsArgs verifies /web-setup tolerates optional free-form argument text.
func TestWebSetupCommandExecuteAcceptsArgs(t *testing.T) {
	result, err := WebSetupCommand{}.Execute(context.Background(), command.Args{RawLine: "github"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != webSetupCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, webSetupCommandFallback)
	}
}
