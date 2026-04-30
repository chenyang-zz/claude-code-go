package bundled

import (
	"context"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/internal/services/tools/skill"
)

func TestInitBundledSkills(t *testing.T) {
	skill.ClearBundledSkills()
	InitBundledSkills()

	skills := skill.GetBundledSkills()
	if len(skills) != 13 {
		t.Errorf("expected 13 bundled skills, got %d", len(skills))
	}

	// Verify each skill can execute without panic
	for _, s := range skills {
		name := s.Metadata().Name
		if name == "" {
			t.Error("skill has empty name")
			continue
		}
		result, err := s.Execute(context.Background(), command.Args{})
		if err != nil {
			t.Errorf("skill %q Execute() returned error: %v", name, err)
		}
		if result.Output == "" {
			t.Errorf("skill %q returned empty output", name)
		}
	}
}

func TestInitBundledSkills_Idempotent(t *testing.T) {
	skill.ClearBundledSkills()
	InitBundledSkills()
	first := len(skill.GetBundledSkills())

	InitBundledSkills()
	second := len(skill.GetBundledSkills())

	if second != first*2 {
		t.Errorf("expected %d skills after double init (register is additive), got %d", first*2, second)
	}
}

func TestInitBundledSkills_ExpectedSkillNames(t *testing.T) {
	skill.ClearBundledSkills()
	InitBundledSkills()

	expected := map[string]bool{
		"lorem-ipsum":    true,
		"simplify":       true,
		"stuck":          true,
		"loop":           true,
		"batch":          true,
		"debug":          true,
		"remember":       true,
		"update-config":  true,
		"keybindings-help": true,
		"verify":         true,
		"skillify":       true,
		"claude-api":     true,
		"schedule":       true,
	}

	found := make(map[string]bool)
	for _, s := range skill.GetBundledSkills() {
		found[s.Metadata().Name] = true
	}

	for name := range expected {
		if !found[name] {
			t.Errorf("expected skill %q not found in registry", name)
		}
	}

	for name := range found {
		if !expected[name] {
			t.Errorf("unexpected skill %q found in registry", name)
		}
	}
}
