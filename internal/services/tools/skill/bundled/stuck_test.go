package bundled

import (
	"context"
	"strings"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/internal/services/tools/skill"
)

func TestStuckSkill(t *testing.T) {
	skill.ClearBundledSkills()
	registerStuckSkill()

	skills := skill.GetBundledSkills()
	if len(skills) != 1 {
		t.Fatalf("expected 1 bundled skill, got %d", len(skills))
	}

	s := skills[0]
	if s.Metadata().Name != "stuck" {
		t.Errorf("expected name 'stuck', got %q", s.Metadata().Name)
	}

	result, err := s.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if !strings.Contains(result.Output, "/stuck") {
		t.Error("expected output to contain /stuck")
	}

	result, err = s.Execute(context.Background(), command.Args{RawLine: "PID 12345"})
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if !strings.Contains(result.Output, "PID 12345") {
		t.Error("expected output to contain user-provided context")
	}
}
