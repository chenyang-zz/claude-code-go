package commands

import (
	"context"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestMemoryCommandMetadata verifies /memory exposes stable metadata.
func TestMemoryCommandMetadata(t *testing.T) {
	meta := MemoryCommand{}.Metadata()
	if meta.Name != "memory" {
		t.Fatalf("Metadata().Name = %q, want memory", meta.Name)
	}
	if meta.Description != "Edit Claude memory files" {
		t.Fatalf("Metadata().Description = %q, want stable memory description", meta.Description)
	}
	if meta.Usage != "/memory" {
		t.Fatalf("Metadata().Usage = %q, want memory usage", meta.Usage)
	}
}

// TestMemoryCommandExecuteReportsFallback verifies /memory reports the current Go host fallback before memory editing is migrated.
func TestMemoryCommandExecuteReportsFallback(t *testing.T) {
	result, err := MemoryCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	want := "Memory file editing is not available in Claude Code Go yet. Memory file discovery, interactive selection, file creation, and editor launch remain unmigrated."
	if result.Output != want {
		t.Fatalf("Execute() output = %q, want %q", result.Output, want)
	}
}
