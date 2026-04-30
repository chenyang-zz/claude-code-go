package bundled

import (
	"context"
	"strings"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/internal/services/tools/skill"
)

func TestSimplifySkill(t *testing.T) {
	skill.ClearBundledSkills()
	registerSimplifySkill()

	skills := skill.GetBundledSkills()
	if len(skills) != 1 {
		t.Fatalf("expected 1 bundled skill, got %d", len(skills))
	}

	s := skills[0]
	if s.Metadata().Name != "simplify" {
		t.Errorf("expected name 'simplify', got %q", s.Metadata().Name)
	}

	// Without args
	result, err := s.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if !strings.Contains(result.Output, "Simplify: Code Review and Cleanup") {
		t.Error("expected output to contain simplify header")
	}

	// With args
	result, err = s.Execute(context.Background(), command.Args{RawLine: "focus on performance"})
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if !strings.Contains(result.Output, "## Additional Focus") {
		t.Error("expected output to contain Additional Focus section")
	}
}
