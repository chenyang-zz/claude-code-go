package builtin

import (
	"os"
	"strings"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/agent"
	"github.com/sheepzhao/claude-code-go/internal/core/tool"
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
	// Enable all feature flags so all agents are registered.
	t.Setenv("CLAUDE_FEATURE_BUILTIN_EXPLORE_PLAN_AGENTS", "1")
	t.Setenv("CLAUDE_FEATURE_VERIFICATION_AGENT", "1")

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
		"claude-code-guide": false,
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

func TestRegisterBuiltInAgents_DefaultRegistersOnlyUnconditional(t *testing.T) {
	// Ensure feature flags are NOT set.
	os.Unsetenv("CLAUDE_FEATURE_BUILTIN_EXPLORE_PLAN_AGENTS")
	os.Unsetenv("CLAUDE_FEATURE_VERIFICATION_AGENT")

	reg := &mockRegistry{}
	if err := RegisterBuiltInAgents(reg); err != nil {
		t.Fatalf("RegisterBuiltInAgents failed: %v", err)
	}

	wantTypes := map[string]bool{
		"general-purpose":   false,
		"statusline-setup":  false,
		"claude-code-guide": false,
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
	}

	for agentType, found := range wantTypes {
		if !found {
			t.Errorf("agent %q was not registered", agentType)
		}
	}
}

func TestRegisterBuiltInAgents_ExploreHasCorrectFields(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_BUILTIN_EXPLORE_PLAN_AGENTS", "1")

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
	t.Setenv("CLAUDE_FEATURE_BUILTIN_EXPLORE_PLAN_AGENTS", "1")

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
	t.Setenv("CLAUDE_FEATURE_VERIFICATION_AGENT", "1")

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

func TestRegisterBuiltInAgents_ClaudeCodeGuideHasCorrectFields(t *testing.T) {
	reg := &mockRegistry{}
	if err := RegisterBuiltInAgents(reg); err != nil {
		t.Fatalf("RegisterBuiltInAgents failed: %v", err)
	}

	var guideDef *agent.Definition
	for i := range reg.defs {
		if reg.defs[i].AgentType == "claude-code-guide" {
			guideDef = &reg.defs[i]
			break
		}
	}
	if guideDef == nil {
		t.Fatal("claude-code-guide agent not found in registry")
	}

	if guideDef.Model != "haiku" {
		t.Errorf("claude-code-guide Model = %q, want %q", guideDef.Model, "haiku")
	}
	if guideDef.PermissionMode != "dontAsk" {
		t.Errorf("claude-code-guide PermissionMode = %q, want %q", guideDef.PermissionMode, "dontAsk")
	}
	wantTools := []string{"Glob", "Grep", "Read", "WebFetch"}
	if len(guideDef.Tools) != len(wantTools) {
		t.Errorf("claude-code-guide Tools = %v, want %v", guideDef.Tools, wantTools)
	}
	for i, tool := range wantTools {
		if i >= len(guideDef.Tools) || guideDef.Tools[i] != tool {
			t.Errorf("claude-code-guide Tools[%d] = %q, want %q", i, guideDef.Tools[i], tool)
		}
	}
	if guideDef.SystemPromptProvider == nil {
		t.Error("claude-code-guide SystemPromptProvider is nil")
	}
	sp := guideDef.SystemPromptProvider.GetSystemPrompt(tool.UseContext{})
	if sp == "" {
		t.Error("claude-code-guide system prompt is empty")
	}
	if !strings.Contains(sp, "Claude guide agent") {
		t.Errorf("claude-code-guide system prompt missing expected content")
	}
}
