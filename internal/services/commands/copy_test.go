package commands

import (
	"context"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestCopyCommandMetadata verifies /copy exposes stable metadata.
func TestCopyCommandMetadata(t *testing.T) {
	meta := CopyCommand{}.Metadata()
	if meta.Name != "copy" {
		t.Fatalf("Metadata().Name = %q, want copy", meta.Name)
	}
	if meta.Description != "Copy Claude's last response to clipboard (or /copy N for the Nth-latest)" {
		t.Fatalf("Metadata().Description = %q, want stable copy description", meta.Description)
	}
	if meta.Usage != "/copy [N]" {
		t.Fatalf("Metadata().Usage = %q, want /copy [N]", meta.Usage)
	}
}

// TestCopyCommandExecuteReportsFallback verifies /copy returns the stable clipboard fallback.
func TestCopyCommandExecuteReportsFallback(t *testing.T) {
	result, err := CopyCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != copyCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, copyCommandFallback)
	}
}
