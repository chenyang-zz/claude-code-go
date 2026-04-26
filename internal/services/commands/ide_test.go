package commands

import (
	"context"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestIdeCommandMetadata verifies /ide is exposed with the expected canonical descriptor.
func TestIdeCommandMetadata(t *testing.T) {
	meta := IdeCommand{}.Metadata()

	if meta.Name != "ide" {
		t.Fatalf("Metadata().Name = %q, want ide", meta.Name)
	}
	if meta.Description != "Manage IDE integrations and show status" {
		t.Fatalf("Metadata().Description = %q, want ide description", meta.Description)
	}
	if meta.Usage != "/ide [open]" {
		t.Fatalf("Metadata().Usage = %q, want /ide [open]", meta.Usage)
	}
}

// TestIdeCommandExecute verifies /ide returns the stable fallback.
func TestIdeCommandExecute(t *testing.T) {
	result, err := IdeCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != ideCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, ideCommandFallback)
	}
}
