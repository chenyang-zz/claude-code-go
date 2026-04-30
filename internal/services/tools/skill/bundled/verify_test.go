package bundled

import (
	"context"
	"strings"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/internal/services/tools/skill"
)

func TestVerifySkill(t *testing.T) {
	skill.ClearBundledSkills()
	registerVerifySkill()

	skills := skill.GetBundledSkills()
	if len(skills) != 1 {
		t.Fatalf("expected 1 bundled skill, got %d", len(skills))
	}

	s := skills[0]
	if s.Metadata().Name != "verify" {
		t.Errorf("expected name 'verify', got %q", s.Metadata().Name)
	}

	result, err := s.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if !strings.Contains(result.Output, "Verify Skill") {
		t.Error("expected output to contain Verify Skill")
	}
	if !strings.Contains(result.Output, "git diff") {
		t.Error("expected output to reference git diff")
	}

	result, err = s.Execute(context.Background(), command.Args{RawLine: "check the login flow"})
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if !strings.Contains(result.Output, "check the login flow") {
		t.Error("expected output to contain user request")
	}
}
