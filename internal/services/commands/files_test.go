package commands

import (
	"context"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestFilesCommandMetadata verifies /files exposes stable metadata.
func TestFilesCommandMetadata(t *testing.T) {
	meta := FilesCommand{}.Metadata()
	if meta.Name != "files" {
		t.Fatalf("Metadata().Name = %q, want files", meta.Name)
	}
	if meta.Description != "List all files currently in context" {
		t.Fatalf("Metadata().Description = %q, want stable files description", meta.Description)
	}
	if meta.Usage != "/files" {
		t.Fatalf("Metadata().Usage = %q, want /files", meta.Usage)
	}
}

// TestFilesCommandExecuteReportsFallback verifies /files returns the stable file-context fallback.
func TestFilesCommandExecuteReportsFallback(t *testing.T) {
	result, err := FilesCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != filesCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, filesCommandFallback)
	}
}
