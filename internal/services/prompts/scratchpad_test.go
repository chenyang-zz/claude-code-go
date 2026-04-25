package prompts

import (
	"context"
	"os"
	"strings"
	"testing"
)

// TestScratchpadSection_Compute verifies the scratchpad prompt is generated with a stable directory.
func TestScratchpadSection_Compute(t *testing.T) {
	section := ScratchpadSection{}
	ctx := WithRuntimeContext(context.Background(), RuntimeContext{
		WorkingDir: t.TempDir(),
		SessionID:  "session-123",
	})

	got, err := section.Compute(ctx)
	if err != nil {
		t.Fatalf("Compute() error = %v", err)
	}
	if !strings.Contains(got, "# Scratchpad Directory") {
		t.Fatalf("Compute() = %q, want scratchpad header", got)
	}
	if !strings.Contains(got, "session-123") {
		t.Fatalf("Compute() = %q, want session-specific directory", got)
	}
	if !strings.Contains(got, os.TempDir()) {
		t.Fatalf("Compute() = %q, want temp directory guidance", got)
	}
}
