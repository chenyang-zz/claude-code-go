package commands

import (
	"context"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestInitCommandMetadata verifies /init is exposed with the expected canonical descriptor.
func TestInitCommandMetadata(t *testing.T) {
	meta := InitCommand{}.Metadata()

	if meta.Name != "init" {
		t.Fatalf("Metadata().Name = %q, want init", meta.Name)
	}
	if meta.Description != "Initialize a new CLAUDE.md file with codebase documentation" {
		t.Fatalf("Metadata().Description = %q, want init description", meta.Description)
	}
	if meta.Usage != "/init" {
		t.Fatalf("Metadata().Usage = %q, want /init", meta.Usage)
	}
}

// TestInitCommandExecute verifies /init returns the stable fallback.
func TestInitCommandExecute(t *testing.T) {
	result, err := InitCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != initCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, initCommandFallback)
	}
}
