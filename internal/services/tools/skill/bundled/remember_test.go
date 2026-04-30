package bundled

import (
	"context"
	"strings"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/internal/services/tools/skill"
)

func TestRememberSkill(t *testing.T) {
	skill.ClearBundledSkills()
	registerRememberSkill()

	skills := skill.GetBundledSkills()
	if len(skills) != 1 {
		t.Fatalf("expected 1 bundled skill, got %d", len(skills))
	}

	s := skills[0]
	if s.Metadata().Name != "remember" {
		t.Errorf("expected name 'remember', got %q", s.Metadata().Name)
	}

	result, err := s.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if !strings.Contains(result.Output, "Memory Review") {
		t.Error("expected output to contain Memory Review")
	}

	result, err = s.Execute(context.Background(), command.Args{RawLine: "check my python conventions"})
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if !strings.Contains(result.Output, "check my python conventions") {
		t.Error("expected output to contain user context")
	}
}
