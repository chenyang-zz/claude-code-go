package commands

import (
	"context"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestStatsCommandMetadata verifies /stats is exposed with the expected canonical descriptor.
func TestStatsCommandMetadata(t *testing.T) {
	meta := StatsCommand{}.Metadata()
	if meta.Name != "stats" {
		t.Fatalf("Metadata().Name = %q, want stats", meta.Name)
	}
	if meta.Description != "Show your Claude Code usage statistics and activity" {
		t.Fatalf("Metadata().Description = %q, want stats description", meta.Description)
	}
	if meta.Usage != "/stats" {
		t.Fatalf("Metadata().Usage = %q, want /stats", meta.Usage)
	}
}

// TestStatsCommandExecute verifies /stats returns the stable usage-statistics fallback.
func TestStatsCommandExecute(t *testing.T) {
	result, err := StatsCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Output != statsCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, statsCommandFallback)
	}
}
