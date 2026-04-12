package commands

import (
	"context"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestReleaseNotesCommandMetadata verifies /release-notes exposes stable metadata.
func TestReleaseNotesCommandMetadata(t *testing.T) {
	meta := ReleaseNotesCommand{}.Metadata()
	if meta.Name != "release-notes" {
		t.Fatalf("Metadata().Name = %q, want release-notes", meta.Name)
	}
	if meta.Description != "View release notes" {
		t.Fatalf("Metadata().Description = %q, want stable release-notes description", meta.Description)
	}
	if meta.Usage != "/release-notes" {
		t.Fatalf("Metadata().Usage = %q, want /release-notes", meta.Usage)
	}
}

// TestReleaseNotesCommandExecuteReportsFallback verifies /release-notes returns the stable changelog fallback.
func TestReleaseNotesCommandExecuteReportsFallback(t *testing.T) {
	result, err := ReleaseNotesCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != releaseNotesCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, releaseNotesCommandFallback)
	}
}
