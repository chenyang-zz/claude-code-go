package commands

import (
	"context"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestHooksCommandMetadata verifies /hooks is exposed with the expected canonical descriptor.
func TestHooksCommandMetadata(t *testing.T) {
	meta := HooksCommand{}.Metadata()

	if meta.Name != "hooks" {
		t.Fatalf("Metadata().Name = %q, want hooks", meta.Name)
	}
	if meta.Description != "View hook configurations for tool events" {
		t.Fatalf("Metadata().Description = %q, want hooks description", meta.Description)
	}
	if meta.Usage != "/hooks" {
		t.Fatalf("Metadata().Usage = %q, want /hooks", meta.Usage)
	}
}

// TestHooksCommandExecute verifies /hooks returns the stable settings fallback.
func TestHooksCommandExecute(t *testing.T) {
	result, err := HooksCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != hooksCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, hooksCommandFallback)
	}
}
