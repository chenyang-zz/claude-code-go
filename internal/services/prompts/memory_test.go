package prompts

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMemorySection_Compute verifies CLAUDE.md content is surfaced in the prompt.
func TestMemorySection_Compute(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("remember the plan\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	section := MemorySection{}
	ctx := WithRuntimeContext(context.Background(), RuntimeContext{WorkingDir: dir})

	got, err := section.Compute(ctx)
	if err != nil {
		t.Fatalf("Compute() error = %v", err)
	}
	if !strings.Contains(got, "# Memory") {
		t.Fatalf("Compute() = %q, want memory header", got)
	}
	if !strings.Contains(got, "remember the plan") {
		t.Fatalf("Compute() = %q, want file contents", got)
	}
}
