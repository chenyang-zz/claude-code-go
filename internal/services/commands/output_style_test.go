package commands

import (
	"context"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestOutputStyleCommandMetadata verifies /output-style exposes stable metadata.
func TestOutputStyleCommandMetadata(t *testing.T) {
	meta := OutputStyleCommand{}.Metadata()
	if meta.Name != "output-style" {
		t.Fatalf("Metadata().Name = %q, want output-style", meta.Name)
	}
	if meta.Description != "Deprecated: use /config to change output style" {
		t.Fatalf("Metadata().Description = %q, want stable output-style description", meta.Description)
	}
	if meta.Usage != "/output-style" {
		t.Fatalf("Metadata().Usage = %q, want explicit output-style usage", meta.Usage)
	}
}

// TestOutputStyleCommandExecuteReportsDeprecation verifies /output-style returns the stable deprecated notice.
func TestOutputStyleCommandExecuteReportsDeprecation(t *testing.T) {
	result, err := OutputStyleCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	want := "/output-style has been deprecated. Use /config to change your output style, or set it in your settings file. Changes take effect on the next session."
	if result.Output != want {
		t.Fatalf("Execute() output = %q, want %q", result.Output, want)
	}
}
