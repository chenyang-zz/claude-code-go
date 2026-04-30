package bundled

import (
	"context"
	"strings"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/internal/services/tools/skill"
)

func TestDebugSkill(t *testing.T) {
	skill.ClearBundledSkills()
	registerDebugSkill()

	skills := skill.GetBundledSkills()
	if len(skills) != 1 {
		t.Fatalf("expected 1 bundled skill, got %d", len(skills))
	}

	s := skills[0]
	if s.Metadata().Name != "debug" {
		t.Errorf("expected name 'debug', got %q", s.Metadata().Name)
	}

	result, err := s.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if !strings.Contains(result.Output, "Debug Skill") {
		t.Error("expected output to contain Debug Skill")
	}
	if !strings.Contains(result.Output, "settings.json") {
		t.Error("expected output to reference settings paths")
	}

	result, err = s.Execute(context.Background(), command.Args{RawLine: "my app keeps crashing"})
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if !strings.Contains(result.Output, "my app keeps crashing") {
		t.Error("expected output to contain issue description")
	}
}
