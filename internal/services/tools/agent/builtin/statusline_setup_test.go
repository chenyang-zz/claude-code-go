package builtin

import (
	"strings"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/agent"
	"github.com/sheepzhao/claude-code-go/internal/core/tool"
)

func TestStatuslineSetupAgentDefinition(t *testing.T) {
	def := StatuslineSetupAgentDefinition
	if def.AgentType != StatuslineSetupAgentType {
		t.Fatalf("AgentType = %q, want %q", def.AgentType, StatuslineSetupAgentType)
	}
	if def.Source != "built-in" {
		t.Fatalf("Source = %q, want built-in", def.Source)
	}
	if def.BaseDir != "built-in" {
		t.Fatalf("BaseDir = %q, want built-in", def.BaseDir)
	}
	if def.WhenToUse == "" {
		t.Fatal("WhenToUse should not be empty")
	}
	if len(def.Tools) != 2 || def.Tools[0] != "Read" || def.Tools[1] != "Edit" {
		t.Fatalf("Tools = %v, want [Read Edit]", def.Tools)
	}
	if def.Model != "sonnet" {
		t.Fatalf("Model = %q, want sonnet", def.Model)
	}
	if def.SystemPromptProvider == nil {
		t.Fatal("SystemPromptProvider should not be nil")
	}
}

func TestStatuslineSetupSystemPrompt(t *testing.T) {
	provider := StatuslineSetupSystemPromptProvider{}
	prompt := provider.GetSystemPrompt(tool.UseContext{})
	if prompt == "" {
		t.Fatal("system prompt should not be empty")
	}
	if !strings.Contains(prompt, "status line setup agent") {
		t.Fatal("system prompt should contain status line setup agent identity")
	}
	if !strings.Contains(prompt, "PS1") {
		t.Fatal("system prompt should contain PS1 conversion instructions")
	}
	if !strings.Contains(prompt, "settings.json") {
		t.Fatal("system prompt should contain settings.json update instructions")
	}
	if !strings.Contains(prompt, "~/.claude/settings.json") {
		t.Fatal("system prompt should mention ~/.claude/settings.json")
	}
}

func TestStatuslineSetupAgentTypeConstant(t *testing.T) {
	if StatuslineSetupAgentType != "statusline-setup" {
		t.Fatalf("StatuslineSetupAgentType = %q, want statusline-setup", StatuslineSetupAgentType)
	}
}

func TestStatuslineSetupIsBuiltIn(t *testing.T) {
	def := StatuslineSetupAgentDefinition
	if !def.Definition.IsBuiltIn() {
		t.Fatal("expected StatuslineSetupAgentDefinition to be built-in")
	}
}

func TestStatuslineSetupRegistryCompatibility(t *testing.T) {
	// Verify the definition can be registered into a minimal registry.
	reg := agent.NewInMemoryRegistry()
	if err := RegisterBuiltInAgents(reg); err != nil {
		t.Fatalf("RegisterBuiltInAgents error: %v", err)
	}

	found, ok := reg.Get(StatuslineSetupAgentType)
	if !ok {
		t.Fatal("expected StatuslineSetupAgent to be registered")
	}
	if found.AgentType != StatuslineSetupAgentType {
		t.Fatalf("registered AgentType = %q, want %q", found.AgentType, StatuslineSetupAgentType)
	}
	if found.SystemPromptProvider == nil {
		t.Fatal("registered SystemPromptProvider should not be nil")
	}
}
