package commands

import (
	"context"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestVersionCommandMetadata verifies /version exposes stable metadata.
func TestVersionCommandMetadata(t *testing.T) {
	meta := VersionCommand{}.Metadata()
	if meta.Name != "version" {
		t.Fatalf("Metadata().Name = %q, want version", meta.Name)
	}
	if meta.Description != "Print the version this session is running (not what autoupdate downloaded)" {
		t.Fatalf("Metadata().Description = %q, want stable version description", meta.Description)
	}
	if meta.Usage != "/version" {
		t.Fatalf("Metadata().Usage = %q, want /version", meta.Usage)
	}
}

// TestVersionCommandExecuteReportsBuildVersion verifies /version returns non-empty build information.
func TestVersionCommandExecuteReportsBuildVersion(t *testing.T) {
	result, err := VersionCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Output == "" {
		t.Fatal("Execute() output = empty, want non-empty version string")
	}
}
