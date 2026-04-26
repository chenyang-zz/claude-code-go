package commands

import (
	"context"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestInstallCommandMetadata verifies /install is exposed with the expected canonical descriptor.
func TestInstallCommandMetadata(t *testing.T) {
	meta := InstallCommand{}.Metadata()

	if meta.Name != "install" {
		t.Fatalf("Metadata().Name = %q, want install", meta.Name)
	}
	if meta.Description != "Install Claude Code native build" {
		t.Fatalf("Metadata().Description = %q, want install description", meta.Description)
	}
	if meta.Usage != "/install [options]" {
		t.Fatalf("Metadata().Usage = %q, want /install [options]", meta.Usage)
	}
}

// TestInstallCommandExecute verifies /install returns the stable fallback.
func TestInstallCommandExecute(t *testing.T) {
	result, err := InstallCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != installCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, installCommandFallback)
	}
}
