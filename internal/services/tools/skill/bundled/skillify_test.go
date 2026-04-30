package bundled

import (
	"context"
	"strings"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/internal/services/tools/skill"
)

func TestSkillifySkill(t *testing.T) {
	skill.ClearBundledSkills()
	registerSkillifySkill()

	skills := skill.GetBundledSkills()
	if len(skills) != 1 {
		t.Fatalf("expected 1 bundled skill, got %d", len(skills))
	}

	s := skills[0]
	if s.Metadata().Name != "skillify" {
		t.Errorf("expected name 'skillify', got %q", s.Metadata().Name)
	}

	result, err := s.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if !strings.Contains(result.Output, "Skillify") {
		t.Error("expected output to contain Skillify")
	}
	if !strings.Contains(result.Output, "SKILL.md") {
		t.Error("expected output to reference SKILL.md format")
	}

	result, err = s.Execute(context.Background(), command.Args{RawLine: "code review workflow"})
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if !strings.Contains(result.Output, "code review workflow") {
		t.Error("expected output to contain process description")
	}
}
