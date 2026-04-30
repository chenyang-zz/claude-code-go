package bundled

import (
	"context"
	"strings"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/internal/services/tools/skill"
)

func TestKeybindingsSkill(t *testing.T) {
	skill.ClearBundledSkills()
	registerKeybindingsSkill()

	skills := skill.GetBundledSkills()
	if len(skills) != 1 {
		t.Fatalf("expected 1 bundled skill, got %d", len(skills))
	}

	s := skills[0]
	if s.Metadata().Name != "keybindings-help" {
		t.Errorf("expected name 'keybindings-help', got %q", s.Metadata().Name)
	}

	result, err := s.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if !strings.Contains(result.Output, "Keybindings Skill") {
		t.Error("expected output to contain Keybindings Skill")
	}
	if !strings.Contains(result.Output, "keybindings.json") {
		t.Error("expected output to reference keybindings.json")
	}
	if !strings.Contains(result.Output, "Available Contexts") {
		t.Error("expected output to contain Available Contexts")
	}

	result, err = s.Execute(context.Background(), command.Args{RawLine: "rebind ctrl+s"})
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if !strings.Contains(result.Output, "rebind ctrl+s") {
		t.Error("expected output to contain user request")
	}
}
