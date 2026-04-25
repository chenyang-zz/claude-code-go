package builtin

import (
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/agent"
)

// mockRegistry is a test double that records registered definitions.
type mockRegistry struct {
	defs []agent.Definition
}

func (m *mockRegistry) Register(def agent.Definition) error {
	m.defs = append(m.defs, def)
	return nil
}

func (m *mockRegistry) Get(agentType string) (agent.Definition, bool) {
	for _, def := range m.defs {
		if def.AgentType == agentType {
			return def, true
		}
	}
	return agent.Definition{}, false
}

func (m *mockRegistry) List() []agent.Definition {
	return m.defs
}

func (m *mockRegistry) Remove(agentType string) bool {
	for i, def := range m.defs {
		if def.AgentType == agentType {
			m.defs = append(m.defs[:i], m.defs[i+1:]...)
			return true
		}
	}
	return false
}

func TestRegisterBuiltInAgents_RegistersAll(t *testing.T) {
	reg := &mockRegistry{}
	if err := RegisterBuiltInAgents(reg); err != nil {
		t.Fatalf("RegisterBuiltInAgents failed: %v", err)
	}

	wantTypes := map[string]bool{
		"Explore":           false,
		"general-purpose":   false,
		"Plan":              false,
		"verification":      false,
		"statusline-setup":  false,
	}

	if len(reg.defs) != len(wantTypes) {
		t.Fatalf("registered %d agents, want %d", len(reg.defs), len(wantTypes))
	}

	for _, def := range reg.defs {
		if _, ok := wantTypes[def.AgentType]; !ok {
			t.Errorf("unexpected agent type registered: %q", def.AgentType)
			continue
		}
		wantTypes[def.AgentType] = true

		if !def.IsBuiltIn() {
			t.Errorf("agent %q: IsBuiltIn() = false, want true", def.AgentType)
		}
		if def.Source != "built-in" {
			t.Errorf("agent %q: Source = %q, want %q", def.AgentType, def.Source, "built-in")
		}
		if def.SystemPromptProvider == nil {
			t.Errorf("agent %q: SystemPromptProvider is nil", def.AgentType)
		}
	}

	for agentType, found := range wantTypes {
		if !found {
			t.Errorf("agent %q was not registered", agentType)
		}
	}
}

func TestRegisterBuiltInAgents_ExploreHasCorrectFields(t *testing.T) {
	reg := &mockRegistry{}
	if err := RegisterBuiltInAgents(reg); err != nil {
		t.Fatalf("RegisterBuiltInAgents failed: %v", err)
	}

	var exploreDef *agent.Definition
	for i := range reg.defs {
		if reg.defs[i].AgentType == "Explore" {
			exploreDef = &reg.defs[i]
			break
		}
	}
	if exploreDef == nil {
		t.Fatal("Explore agent not found in registry")
	}

	if exploreDef.Model != "haiku" {
		t.Errorf("Explore Model = %q, want %q", exploreDef.Model, "haiku")
	}
	if !exploreDef.OmitClaudeMd {
		t.Error("Explore OmitClaudeMd = false, want true")
	}
}

func TestRegisterBuiltInAgents_GeneralPurposeHasCorrectFields(t *testing.T) {
	reg := &mockRegistry{}
	if err := RegisterBuiltInAgents(reg); err != nil {
		t.Fatalf("RegisterBuiltInAgents failed: %v", err)
	}

	var gpDef *agent.Definition
	for i := range reg.defs {
		if reg.defs[i].AgentType == "general-purpose" {
			gpDef = &reg.defs[i]
			break
		}
	}
	if gpDef == nil {
		t.Fatal("general-purpose agent not found in registry")
	}

	if len(gpDef.Tools) != 1 || gpDef.Tools[0] != "*" {
		t.Errorf("general-purpose Tools = %v, want [\"*\"]", gpDef.Tools)
	}
	if gpDef.OmitClaudeMd {
		t.Error("general-purpose OmitClaudeMd = true, want false")
	}
}

func TestRegisterBuiltInAgents_PlanHasCorrectFields(t *testing.T) {
	reg := &mockRegistry{}
	if err := RegisterBuiltInAgents(reg); err != nil {
		t.Fatalf("RegisterBuiltInAgents failed: %v", err)
	}

	var planDef *agent.Definition
	for i := range reg.defs {
		if reg.defs[i].AgentType == "Plan" {
			planDef = &reg.defs[i]
			break
		}
	}
	if planDef == nil {
		t.Fatal("Plan agent not found in registry")
	}

	if !planDef.OmitClaudeMd {
		t.Error("Plan OmitClaudeMd = false, want true")
	}
}

func TestRegisterBuiltInAgents_VerificationHasCorrectFields(t *testing.T) {
	reg := &mockRegistry{}
	if err := RegisterBuiltInAgents(reg); err != nil {
		t.Fatalf("RegisterBuiltInAgents failed: %v", err)
	}

	var verDef *agent.Definition
	for i := range reg.defs {
		if reg.defs[i].AgentType == "verification" {
			verDef = &reg.defs[i]
			break
		}
	}
	if verDef == nil {
		t.Fatal("verification agent not found in registry")
	}

	if !verDef.Background {
		t.Error("verification Background = false, want true")
	}
	if verDef.CriticalSystemReminder == "" {
		t.Error("verification CriticalSystemReminder is empty")
	}
}
