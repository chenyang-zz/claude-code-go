package commands

import (
	"context"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestTagCommandMetadata verifies /tag is exposed with the expected canonical descriptor.
func TestTagCommandMetadata(t *testing.T) {
	meta := TagCommand{}.Metadata()

	if meta.Name != "tag" {
		t.Fatalf("Metadata().Name = %q, want tag", meta.Name)
	}
	if meta.Description != "Toggle a searchable tag on the current session" {
		t.Fatalf("Metadata().Description = %q, want tag description", meta.Description)
	}
	if meta.Usage != "/tag <tag-name>" {
		t.Fatalf("Metadata().Usage = %q, want /tag <tag-name>", meta.Usage)
	}
}

// TestTagCommandExecuteRequiresTagName verifies /tag rejects empty input with usage guidance.
func TestTagCommandExecuteRequiresTagName(t *testing.T) {
	_, err := TagCommand{}.Execute(context.Background(), command.Args{})
	if err == nil {
		t.Fatal("Execute() error = nil, want usage error")
	}
	if got := err.Error(); got != "usage: /tag <tag-name>" {
		t.Fatalf("Execute() error = %q, want usage guidance", got)
	}
}

// TestTagCommandExecute verifies /tag returns the stable fallback when tag name is provided.
func TestTagCommandExecute(t *testing.T) {
	result, err := TagCommand{}.Execute(context.Background(), command.Args{
		RawLine: "backend",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != tagCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, tagCommandFallback)
	}
}
