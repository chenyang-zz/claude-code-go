package bundled

import (
	"context"
	"strings"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/internal/services/tools/skill"
)

func TestUpdateConfigSkill(t *testing.T) {
	skill.ClearBundledSkills()
	registerUpdateConfigSkill()

	skills := skill.GetBundledSkills()
	if len(skills) != 1 {
		t.Fatalf("expected 1 bundled skill, got %d", len(skills))
	}

	s := skills[0]
	if s.Metadata().Name != "update-config" {
		t.Errorf("expected name 'update-config', got %q", s.Metadata().Name)
	}

	result, err := s.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if !strings.Contains(result.Output, "Update Config Skill") {
		t.Error("expected output to contain Update Config Skill")
	}
	if !strings.Contains(result.Output, "settings.json") {
		t.Error("expected output to reference settings.json")
	}
	if !strings.Contains(result.Output, "## Hooks Configuration") {
		t.Error("expected output to contain Hooks Configuration section")
	}

	result, err = s.Execute(context.Background(), command.Args{RawLine: "add npm permission"})
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if !strings.Contains(result.Output, "add npm permission") {
		t.Error("expected output to contain user request")
	}
}
