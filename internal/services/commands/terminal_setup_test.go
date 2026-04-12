package commands

import (
	"context"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestTerminalSetupCommandMetadata verifies /terminal-setup is exposed with the expected canonical descriptor.
func TestTerminalSetupCommandMetadata(t *testing.T) {
	meta := TerminalSetupCommand{}.Metadata()
	if meta.Name != "terminal-setup" {
		t.Fatalf("Metadata().Name = %q, want terminal-setup", meta.Name)
	}
	if meta.Description != "Install Shift+Enter key binding for newlines" {
		t.Fatalf("Metadata().Description = %q, want terminal-setup description", meta.Description)
	}
	if meta.Usage != "/terminal-setup" {
		t.Fatalf("Metadata().Usage = %q, want /terminal-setup", meta.Usage)
	}
}

// TestTerminalSetupCommandExecute verifies /terminal-setup returns the stable terminal guidance fallback.
func TestTerminalSetupCommandExecute(t *testing.T) {
	result, err := TerminalSetupCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Output != terminalSetupCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, terminalSetupCommandFallback)
	}
}
