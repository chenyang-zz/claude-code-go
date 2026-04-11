package commands

import (
	"context"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestCostCommandMetadata verifies /cost exposes stable metadata.
func TestCostCommandMetadata(t *testing.T) {
	meta := CostCommand{}.Metadata()
	if meta.Name != "cost" {
		t.Fatalf("Metadata().Name = %q, want cost", meta.Name)
	}
	if meta.Description != "Show the total cost and duration of the current session" {
		t.Fatalf("Metadata().Description = %q, want stable cost description", meta.Description)
	}
	if meta.Usage != "/cost" {
		t.Fatalf("Metadata().Usage = %q, want /cost", meta.Usage)
	}
}

// TestCostCommandExecute verifies /cost reports the current Go host fallback while usage tracking is unavailable.
func TestCostCommandExecute(t *testing.T) {
	result, err := CostCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	want := "Session cost and duration tracking are not available in Claude Code Go yet."
	if result.Output != want {
		t.Fatalf("Execute() output = %q, want %q", result.Output, want)
	}
}
