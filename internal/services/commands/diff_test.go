package commands

import (
	"context"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestDiffCommandMetadata verifies /diff is exposed with the expected canonical descriptor.
func TestDiffCommandMetadata(t *testing.T) {
	meta := DiffCommand{}.Metadata()
	if meta.Name != "diff" {
		t.Fatalf("Metadata().Name = %q, want diff", meta.Name)
	}
	if meta.Description != "View uncommitted changes and per-turn diffs" {
		t.Fatalf("Metadata().Description = %q, want diff description", meta.Description)
	}
	if meta.Usage != "/diff" {
		t.Fatalf("Metadata().Usage = %q, want /diff", meta.Usage)
	}
}

// TestDiffCommandExecute verifies /diff returns the stable fallback guidance.
func TestDiffCommandExecute(t *testing.T) {
	result, err := DiffCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Output != diffCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, diffCommandFallback)
	}
}
