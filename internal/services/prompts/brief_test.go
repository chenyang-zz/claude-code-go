package prompts

import (
	"context"
	"strings"
	"testing"
)

// TestBriefSection_Compute verifies the brief guidance stays disabled by default.
func TestBriefSection_Compute(t *testing.T) {
	section := BriefSection{}

	got, err := section.Compute(context.Background())
	if err != nil {
		t.Fatalf("Compute() error = %v", err)
	}
	if got != "" {
		t.Fatalf("Compute() = %q, want empty string", got)
	}
}

// TestBriefSection_ComputeEnabled verifies the brief guidance is emitted when enabled.
func TestBriefSection_ComputeEnabled(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_KAIROS_BRIEF", "1")

	section := BriefSection{}
	got, err := section.Compute(context.Background())
	if err != nil {
		t.Fatalf("Compute() error = %v", err)
	}
	if !strings.Contains(got, "# Brief mode") {
		t.Fatalf("Compute() = %q, want brief header", got)
	}
}
