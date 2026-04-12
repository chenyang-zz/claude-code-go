package commands

import (
	"context"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestUsageCommandMetadata verifies /usage is exposed with the expected canonical descriptor.
func TestUsageCommandMetadata(t *testing.T) {
	meta := UsageCommand{}.Metadata()
	if meta.Name != "usage" {
		t.Fatalf("Metadata().Name = %q, want usage", meta.Name)
	}
	if meta.Description != "Show plan usage limits" {
		t.Fatalf("Metadata().Description = %q, want usage description", meta.Description)
	}
	if meta.Usage != "/usage" {
		t.Fatalf("Metadata().Usage = %q, want /usage", meta.Usage)
	}
}

// TestUsageCommandExecute verifies /usage returns the stable usage-limit fallback.
func TestUsageCommandExecute(t *testing.T) {
	result, err := UsageCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Output != usageCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, usageCommandFallback)
	}
}
