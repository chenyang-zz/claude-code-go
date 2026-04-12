package commands

import (
	"context"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestKeybindingsCommandMetadata verifies /keybindings is exposed with the expected canonical descriptor.
func TestKeybindingsCommandMetadata(t *testing.T) {
	meta := KeybindingsCommand{}.Metadata()
	if meta.Name != "keybindings" {
		t.Fatalf("Metadata().Name = %q, want keybindings", meta.Name)
	}
	if meta.Description != "Open or create your keybindings configuration file" {
		t.Fatalf("Metadata().Description = %q, want keybindings description", meta.Description)
	}
	if meta.Usage != "/keybindings" {
		t.Fatalf("Metadata().Usage = %q, want /keybindings", meta.Usage)
	}
}

// TestKeybindingsCommandExecute verifies /keybindings returns the stable keybindings fallback.
func TestKeybindingsCommandExecute(t *testing.T) {
	result, err := KeybindingsCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Output != keybindingsCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, keybindingsCommandFallback)
	}
}
