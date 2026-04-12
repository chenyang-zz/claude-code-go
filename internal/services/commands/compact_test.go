package commands

import (
	"context"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestCompactCommandMetadata verifies /compact exposes stable metadata.
func TestCompactCommandMetadata(t *testing.T) {
	meta := CompactCommand{}.Metadata()
	if meta.Name != "compact" {
		t.Fatalf("Metadata().Name = %q, want compact", meta.Name)
	}
	if meta.Description != "Clear conversation history but keep a summary in context" {
		t.Fatalf("Metadata().Description = %q, want stable compact description", meta.Description)
	}
	if meta.Usage != "/compact [instructions]" {
		t.Fatalf("Metadata().Usage = %q, want compact usage", meta.Usage)
	}
}

// TestCompactCommandExecuteReportsFallback verifies /compact reports the current Go host fallback before compaction is migrated.
func TestCompactCommandExecuteReportsFallback(t *testing.T) {
	result, err := CompactCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	want := "Conversation compaction is not available in Claude Code Go yet. Use /clear to start a new session; summary-preserving compact, custom instructions, and compact hooks remain unmigrated."
	if result.Output != want {
		t.Fatalf("Execute() output = %q, want %q", result.Output, want)
	}
}
