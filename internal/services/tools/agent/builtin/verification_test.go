package builtin

import (
	"strings"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/tool"
)

func TestVerificationAgentDefinition_AgentType(t *testing.T) {
	if VerificationAgentDefinition.AgentType != "verification" {
		t.Errorf("AgentType = %q, want %q", VerificationAgentDefinition.AgentType, "verification")
	}
}

func TestVerificationAgentDefinition_Source(t *testing.T) {
	if VerificationAgentDefinition.Source != "built-in" {
		t.Errorf("Source = %q, want %q", VerificationAgentDefinition.Source, "built-in")
	}
}

func TestVerificationAgentDefinition_IsBuiltIn(t *testing.T) {
	if !VerificationAgentDefinition.Definition.IsBuiltIn() {
		t.Error("IsBuiltIn() = false, want true")
	}
}

func TestVerificationAgentDefinition_Background(t *testing.T) {
	if !VerificationAgentDefinition.Background {
		t.Error("Background = false, want true")
	}
}

func TestVerificationAgentDefinition_DisallowedTools(t *testing.T) {
	want := []string{"Agent", "ExitPlanMode", "Edit", "Write", "NotebookEdit"}
	if len(VerificationAgentDefinition.DisallowedTools) != len(want) {
		t.Fatalf("DisallowedTools length = %d, want %d", len(VerificationAgentDefinition.DisallowedTools), len(want))
	}
	for i, v := range want {
		if VerificationAgentDefinition.DisallowedTools[i] != v {
			t.Errorf("DisallowedTools[%d] = %q, want %q", i, VerificationAgentDefinition.DisallowedTools[i], v)
		}
	}
}

func TestVerificationAgentDefinition_CriticalSystemReminder(t *testing.T) {
	if VerificationAgentDefinition.CriticalSystemReminder == "" {
		t.Fatal("CriticalSystemReminder is empty")
	}
	if !strings.Contains(VerificationAgentDefinition.CriticalSystemReminder, "VERDICT: PASS") {
		t.Error("CriticalSystemReminder does not contain 'VERDICT: PASS'")
	}
	if !strings.Contains(VerificationAgentDefinition.CriticalSystemReminder, "VERDICT: FAIL") {
		t.Error("CriticalSystemReminder does not contain 'VERDICT: FAIL'")
	}
	if !strings.Contains(VerificationAgentDefinition.CriticalSystemReminder, "VERDICT: PARTIAL") {
		t.Error("CriticalSystemReminder does not contain 'VERDICT: PARTIAL'")
	}
}

func TestVerificationSystemPromptProvider_GetSystemPrompt(t *testing.T) {
	provider := VerificationAgentDefinition.SystemPromptProvider
	if provider == nil {
		t.Fatal("SystemPromptProvider is nil")
	}

	prompt := provider.GetSystemPrompt(tool.UseContext{})
	if prompt == "" {
		t.Fatal("GetSystemPrompt returned empty string")
	}

	if !strings.Contains(prompt, "verification specialist") {
		t.Error("system prompt does not contain 'verification specialist' keyword")
	}
	if !strings.Contains(prompt, "VERDICT:") {
		t.Error("system prompt does not contain 'VERDICT:' keyword")
	}
	if !strings.Contains(prompt, "ADVERSARIAL PROBES") {
		t.Error("system prompt does not contain 'ADVERSARIAL PROBES' keyword")
	}
	if !strings.Contains(prompt, "DO NOT MODIFY THE PROJECT") {
		t.Error("system prompt does not contain 'DO NOT MODIFY THE PROJECT' keyword")
	}
	if !strings.Contains(prompt, "RECOGNIZE YOUR OWN RATIONALIZATIONS") {
		t.Error("system prompt does not contain 'RECOGNIZE YOUR OWN RATIONALIZATIONS' keyword")
	}
	if !strings.Contains(prompt, "VERIFICATION STRATEGY") {
		t.Error("system prompt does not contain 'VERIFICATION STRATEGY' keyword")
	}
}

func TestVerificationAgentDefinition_WhenToUse(t *testing.T) {
	if VerificationAgentDefinition.WhenToUse == "" {
		t.Fatal("WhenToUse is empty")
	}
	if !strings.Contains(VerificationAgentDefinition.WhenToUse, "PASS/FAIL/PARTIAL") {
		t.Error("WhenToUse does not contain 'PASS/FAIL/PARTIAL'")
	}
}
