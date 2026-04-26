package commands

import (
	"context"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestContextCommandMetadata verifies /context is exposed with the expected canonical descriptor.
func TestContextCommandMetadata(t *testing.T) {
	meta := ContextCommand{}.Metadata()

	if meta.Name != "context" {
		t.Fatalf("Metadata().Name = %q, want context", meta.Name)
	}
	if meta.Description != "Show current context usage" {
		t.Fatalf("Metadata().Description = %q, want context description", meta.Description)
	}
	if meta.Usage != "/context" {
		t.Fatalf("Metadata().Usage = %q, want /context", meta.Usage)
	}
}

// TestContextCommandExecute verifies /context returns the stable fallback.
func TestContextCommandExecute(t *testing.T) {
	result, err := ContextCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != contextCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, contextCommandFallback)
	}
}
