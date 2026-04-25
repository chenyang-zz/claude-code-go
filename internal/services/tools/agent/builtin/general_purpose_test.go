package builtin

import (
	"strings"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/tool"
)

func TestGeneralPurposeAgentDefinition_AgentType(t *testing.T) {
	if GeneralPurposeAgentDefinition.AgentType != "general-purpose" {
		t.Errorf("AgentType = %q, want %q", GeneralPurposeAgentDefinition.AgentType, "general-purpose")
	}
}

func TestGeneralPurposeAgentDefinition_Source(t *testing.T) {
	if GeneralPurposeAgentDefinition.Source != "built-in" {
		t.Errorf("Source = %q, want %q", GeneralPurposeAgentDefinition.Source, "built-in")
	}
}

func TestGeneralPurposeAgentDefinition_IsBuiltIn(t *testing.T) {
	if !GeneralPurposeAgentDefinition.Definition.IsBuiltIn() {
		t.Error("IsBuiltIn() = false, want true")
	}
}

func TestGeneralPurposeAgentDefinition_WhenToUse(t *testing.T) {
	if GeneralPurposeAgentDefinition.WhenToUse == "" {
		t.Error("WhenToUse is empty")
	}

	want := "General-purpose agent for researching complex questions"
	if !strings.Contains(GeneralPurposeAgentDefinition.WhenToUse, want) {
		t.Errorf("WhenToUse does not contain %q", want)
	}
}

func TestGeneralPurposeSystemPromptProvider_GetSystemPrompt(t *testing.T) {
	provider := GeneralPurposeAgentDefinition.SystemPromptProvider
	if provider == nil {
		t.Fatal("SystemPromptProvider is nil")
	}

	prompt := provider.GetSystemPrompt(tool.UseContext{})
	if prompt == "" {
		t.Fatal("GetSystemPrompt returned empty string")
	}

	if !strings.Contains(prompt, "Claude Code") {
		t.Error("system prompt does not contain 'Claude Code' keyword")
	}

	if !strings.Contains(prompt, "concise report") {
		t.Error("system prompt does not contain 'concise report' keyword")
	}

	if !strings.Contains(prompt, "Searching for code") {
		t.Error("system prompt does not contain 'Searching for code' keyword")
	}

	if !strings.Contains(prompt, "NEVER create files") {
		t.Error("system prompt does not contain 'NEVER create files' keyword")
	}
}
