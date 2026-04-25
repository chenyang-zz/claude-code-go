package prompts

import (
	"context"
	"strings"
	"testing"
)

// TestSessionGuidanceSection_Compute verifies the guidance section is emitted
// only when matching tools are available.
func TestSessionGuidanceSection_Compute(t *testing.T) {
	section := SessionGuidanceSection{}
	ctx := WithRuntimeContext(context.Background(), RuntimeContext{
		EnabledToolNames: map[string]struct{}{
			"Agent":           {},
			"Read":            {},
			"Glob":            {},
			"Bash":            {},
			"AskUserQuestion": {},
			"Skill":           {},
			"DiscoverSkills":  {},
		},
	})

	got, err := section.Compute(ctx)
	if err != nil {
		t.Fatalf("Compute() error = %v", err)
	}

	if !strings.Contains(got, "# Session-specific guidance") {
		t.Fatalf("Compute() = %q, want session guidance header", got)
	}
	if !strings.Contains(got, "Use the Agent tool") {
		t.Fatalf("Compute() = %q, want Agent guidance", got)
	}
	if !strings.Contains(got, "prefer Read") {
		t.Fatalf("Compute() = %q, want Read guidance", got)
	}
	if !strings.Contains(got, "AskUserQuestion") {
		t.Fatalf("Compute() = %q, want AskUserQuestion guidance", got)
	}
	if !strings.Contains(got, "Use the Skill tool") {
		t.Fatalf("Compute() = %q, want Skill guidance", got)
	}
	if !strings.Contains(got, "DiscoverSkills") {
		t.Fatalf("Compute() = %q, want DiscoverSkills guidance", got)
	}
}

// TestSessionGuidanceSection_ComputeEmpty verifies the section is skipped when
// the runtime context does not expose enough tools to warrant guidance.
func TestSessionGuidanceSection_ComputeEmpty(t *testing.T) {
	section := SessionGuidanceSection{}

	got, err := section.Compute(context.Background())
	if err != nil {
		t.Fatalf("Compute() error = %v", err)
	}
	if got != "" {
		t.Fatalf("Compute() = %q, want empty string", got)
	}
}
