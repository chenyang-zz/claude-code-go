package builtin

import (
	"strings"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/tool"
)

func TestExploreAgentDefinition_AgentType(t *testing.T) {
	if ExploreAgentDefinition.AgentType != "Explore" {
		t.Errorf("AgentType = %q, want %q", ExploreAgentDefinition.AgentType, "Explore")
	}
}

func TestExploreAgentDefinition_Source(t *testing.T) {
	if ExploreAgentDefinition.Source != "built-in" {
		t.Errorf("Source = %q, want %q", ExploreAgentDefinition.Source, "built-in")
	}
}

func TestExploreAgentDefinition_OmitClaudeMd(t *testing.T) {
	if !ExploreAgentDefinition.OmitClaudeMd {
		t.Error("OmitClaudeMd = false, want true")
	}
}

func TestExploreAgentDefinition_DisallowedTools(t *testing.T) {
	want := []string{"Agent", "Edit", "Write", "NotebookEdit"}
	if len(ExploreAgentDefinition.DisallowedTools) != len(want) {
		t.Fatalf("DisallowedTools length = %d, want %d", len(ExploreAgentDefinition.DisallowedTools), len(want))
	}
	for i, v := range want {
		if ExploreAgentDefinition.DisallowedTools[i] != v {
			t.Errorf("DisallowedTools[%d] = %q, want %q", i, ExploreAgentDefinition.DisallowedTools[i], v)
		}
	}
}

func TestExploreAgentDefinition_IsBuiltIn(t *testing.T) {
	if !ExploreAgentDefinition.Definition.IsBuiltIn() {
		t.Error("IsBuiltIn() = false, want true")
	}
}

func TestExploreSystemPromptProvider_GetSystemPrompt(t *testing.T) {
	provider := ExploreAgentDefinition.SystemPromptProvider
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
}

func TestExploreAgentDefinition_WhenToUse(t *testing.T) {
	if ExploreAgentDefinition.WhenToUse == "" {
		t.Error("WhenToUse is empty")
	}
}

func TestExploreAgentDefinition_Model(t *testing.T) {
	if ExploreAgentDefinition.Model != "haiku" {
		t.Errorf("Model = %q, want %q", ExploreAgentDefinition.Model, "haiku")
	}
}
