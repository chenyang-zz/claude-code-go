package prompts

import (
	"context"
	"strings"
	"testing"
)

// TestProactiveSection_Compute verifies the proactive guidance stays disabled by default.
func TestProactiveSection_Compute(t *testing.T) {
	section := ProactiveSection{}

	got, err := section.Compute(context.Background())
	if err != nil {
		t.Fatalf("Compute() error = %v", err)
	}
	if got != "" {
		t.Fatalf("Compute() = %q, want empty string", got)
	}
}

// TestProactiveSection_ComputeEnabled verifies the proactive guidance is emitted when enabled.
func TestProactiveSection_ComputeEnabled(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_PROACTIVE", "1")

	section := ProactiveSection{}
	got, err := section.Compute(context.Background())
	if err != nil {
		t.Fatalf("Compute() error = %v", err)
	}
	if !strings.Contains(got, "# Autonomous work") {
		t.Fatalf("Compute() = %q, want proactive header", got)
	}
}
