package commands

import (
	"context"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestExportCommandMetadata verifies /export exposes stable metadata.
func TestExportCommandMetadata(t *testing.T) {
	meta := ExportCommand{}.Metadata()
	if meta.Name != "export" {
		t.Fatalf("Metadata().Name = %q, want export", meta.Name)
	}
	if meta.Description != "Export the current conversation to a file or clipboard" {
		t.Fatalf("Metadata().Description = %q, want stable export description", meta.Description)
	}
	if meta.Usage != "/export [filename]" {
		t.Fatalf("Metadata().Usage = %q, want /export [filename]", meta.Usage)
	}
}

// TestExportCommandExecuteReportsFallback verifies /export returns the stable export fallback.
func TestExportCommandExecuteReportsFallback(t *testing.T) {
	result, err := ExportCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != exportCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, exportCommandFallback)
	}
}
