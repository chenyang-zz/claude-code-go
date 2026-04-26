package commands

import (
	"context"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestInstallGitHubAppCommandMetadata verifies /install-github-app is exposed with the expected canonical descriptor.
func TestInstallGitHubAppCommandMetadata(t *testing.T) {
	meta := InstallGitHubAppCommand{}.Metadata()

	if meta.Name != "install-github-app" {
		t.Fatalf("Metadata().Name = %q, want install-github-app", meta.Name)
	}
	if meta.Description != "Set up Claude GitHub Actions for a repository" {
		t.Fatalf("Metadata().Description = %q, want install-github-app description", meta.Description)
	}
	if meta.Usage != "/install-github-app" {
		t.Fatalf("Metadata().Usage = %q, want /install-github-app", meta.Usage)
	}
}

// TestInstallGitHubAppCommandExecute verifies /install-github-app returns the stable fallback.
func TestInstallGitHubAppCommandExecute(t *testing.T) {
	result, err := InstallGitHubAppCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != installGitHubAppCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, installGitHubAppCommandFallback)
	}
}
