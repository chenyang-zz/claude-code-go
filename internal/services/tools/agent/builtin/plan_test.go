package builtin

import (
	"strings"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/tool"
)

func TestPlanAgentDefinition_AgentType(t *testing.T) {
	if PlanAgentDefinition.AgentType != "Plan" {
		t.Errorf("AgentType = %q, want %q", PlanAgentDefinition.AgentType, "Plan")
	}
}

func TestPlanAgentDefinition_Source(t *testing.T) {
	if PlanAgentDefinition.Source != "built-in" {
		t.Errorf("Source = %q, want %q", PlanAgentDefinition.Source, "built-in")
	}
}

func TestPlanAgentDefinition_OmitClaudeMd(t *testing.T) {
	if !PlanAgentDefinition.OmitClaudeMd {
		t.Error("OmitClaudeMd = false, want true")
	}
}

func TestPlanAgentDefinition_DisallowedTools(t *testing.T) {
	want := []string{"Agent", "ExitPlanMode", "Edit", "Write", "NotebookEdit"}
	if len(PlanAgentDefinition.DisallowedTools) != len(want) {
		t.Fatalf("DisallowedTools length = %d, want %d", len(PlanAgentDefinition.DisallowedTools), len(want))
	}
	for i, v := range want {
		if PlanAgentDefinition.DisallowedTools[i] != v {
			t.Errorf("DisallowedTools[%d] = %q, want %q", i, PlanAgentDefinition.DisallowedTools[i], v)
		}
	}
}

func TestPlanAgentDefinition_IsBuiltIn(t *testing.T) {
	if !PlanAgentDefinition.Definition.IsBuiltIn() {
		t.Error("IsBuiltIn() = false, want true")
	}
}

func TestPlanSystemPromptProvider_GetSystemPrompt(t *testing.T) {
	provider := PlanAgentDefinition.SystemPromptProvider
	if provider == nil {
		t.Fatal("SystemPromptProvider is nil")
	}

	prompt := provider.GetSystemPrompt(tool.UseContext{})
	if prompt == "" {
		t.Fatal("GetSystemPrompt returned empty string")
	}

	if !strings.Contains(prompt, "READ-ONLY MODE") {
		t.Error("system prompt does not contain 'READ-ONLY MODE' keyword")
	}

	if !strings.Contains(prompt, "Critical Files for Implementation") {
		t.Error("system prompt does not contain 'Critical Files for Implementation' keyword")
	}

	if !strings.Contains(prompt, "Glob, Grep, and Read") {
		t.Error("system prompt does not contain 'Glob, Grep, and Read' keyword")
	}
}

func TestPlanAgentDefinition_WhenToUse(t *testing.T) {
	if PlanAgentDefinition.WhenToUse == "" {
		t.Error("WhenToUse is empty")
	}
}
