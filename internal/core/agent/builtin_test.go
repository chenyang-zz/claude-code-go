package agent

import (
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/tool"
)

type mockSystemPromptProvider struct {
	prompt string
}

func (m *mockSystemPromptProvider) GetSystemPrompt(toolCtx tool.UseContext) string {
	return m.prompt
}

func TestNewBuiltInAgentDefinition(t *testing.T) {
	provider := &mockSystemPromptProvider{prompt: "You are an explore agent."}

	def := NewBuiltInAgentDefinition("explore", provider)

	if def.AgentType != "explore" {
		t.Errorf("AgentType = %q, want 'explore'", def.AgentType)
	}
	if def.Source != "built-in" {
		t.Errorf("Source = %q, want 'built-in'", def.Source)
	}
	if def.BaseDir != "built-in" {
		t.Errorf("BaseDir = %q, want 'built-in'", def.BaseDir)
	}
	if def.SystemPromptProvider != provider {
		t.Error("SystemPromptProvider mismatch")
	}
}

func TestNewBuiltInAgentDefinition_NilProvider(t *testing.T) {
	def := NewBuiltInAgentDefinition("explore", nil)

	if def.AgentType != "explore" {
		t.Errorf("AgentType = %q, want 'explore'", def.AgentType)
	}
	if def.Source != "built-in" {
		t.Errorf("Source = %q, want 'built-in'", def.Source)
	}
	if def.BaseDir != "built-in" {
		t.Errorf("BaseDir = %q, want 'built-in'", def.BaseDir)
	}
	if def.SystemPromptProvider != nil {
		t.Error("SystemPromptProvider expected nil")
	}
}

func TestBuiltInAgentDefinition_IsBuiltIn(t *testing.T) {
	provider := &mockSystemPromptProvider{prompt: "test"}
	def := NewBuiltInAgentDefinition("verify", provider)

	if !def.IsBuiltIn() {
		t.Error("expected IsBuiltIn() = true")
	}
	if def.IsPlugin() {
		t.Error("expected IsPlugin() = false")
	}
	if def.IsCustom() {
		t.Error("expected IsCustom() = false")
	}
}
