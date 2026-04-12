package commands

import (
	"context"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestPlanCommandMetadata verifies /plan is exposed with the expected canonical descriptor.
func TestPlanCommandMetadata(t *testing.T) {
	meta := PlanCommand{}.Metadata()
	if meta.Name != "plan" {
		t.Fatalf("Metadata().Name = %q, want plan", meta.Name)
	}
	if meta.Description != "Enable plan mode or view the current session plan" {
		t.Fatalf("Metadata().Description = %q, want plan description", meta.Description)
	}
	if meta.Usage != "/plan [open|<description>]" {
		t.Fatalf("Metadata().Usage = %q, want /plan [open|<description>]", meta.Usage)
	}
}

// TestPlanCommandExecute verifies /plan returns the stable fallback guidance.
func TestPlanCommandExecute(t *testing.T) {
	result, err := PlanCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Output != planCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, planCommandFallback)
	}
}
