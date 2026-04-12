package commands

import (
	"context"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestUpgradeCommandMetadata verifies /upgrade exposes stable metadata.
func TestUpgradeCommandMetadata(t *testing.T) {
	meta := UpgradeCommand{}.Metadata()
	if meta.Name != "upgrade" {
		t.Fatalf("Metadata().Name = %q, want upgrade", meta.Name)
	}
	if meta.Description != "Upgrade to Max for higher rate limits and more Opus" {
		t.Fatalf("Metadata().Description = %q, want stable upgrade description", meta.Description)
	}
	if meta.Usage != "/upgrade" {
		t.Fatalf("Metadata().Usage = %q, want /upgrade", meta.Usage)
	}
}

// TestUpgradeCommandExecuteReportsFallback verifies /upgrade returns the stable upgrade fallback.
func TestUpgradeCommandExecuteReportsFallback(t *testing.T) {
	result, err := UpgradeCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != upgradeCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, upgradeCommandFallback)
	}
}
