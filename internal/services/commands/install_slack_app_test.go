package commands

import (
	"context"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestInstallSlackAppCommandMetadata verifies /install-slack-app is exposed with the expected canonical descriptor.
func TestInstallSlackAppCommandMetadata(t *testing.T) {
	meta := InstallSlackAppCommand{}.Metadata()

	if meta.Name != "install-slack-app" {
		t.Fatalf("Metadata().Name = %q, want install-slack-app", meta.Name)
	}
	if meta.Description != "Install the Claude Slack app" {
		t.Fatalf("Metadata().Description = %q, want install-slack-app description", meta.Description)
	}
	if meta.Usage != "/install-slack-app" {
		t.Fatalf("Metadata().Usage = %q, want /install-slack-app", meta.Usage)
	}
}

// TestInstallSlackAppCommandExecute verifies /install-slack-app returns the stable fallback.
func TestInstallSlackAppCommandExecute(t *testing.T) {
	result, err := InstallSlackAppCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != installSlackAppCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, installSlackAppCommandFallback)
	}
}
