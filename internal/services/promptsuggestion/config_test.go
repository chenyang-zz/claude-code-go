package promptsuggestion

import (
	"os"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/featureflag"
)

func TestIsPromptSuggestionEnabled_EnvTruthy(t *testing.T) {
	os.Setenv("CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION", "1")
	defer os.Unsetenv("CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION")

	if !IsPromptSuggestionEnabled() {
		t.Error("expected true when CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION=1")
	}
}

func TestIsPromptSuggestionEnabled_EnvFalsy(t *testing.T) {
	os.Setenv("CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION", "0")
	defer os.Unsetenv("CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION")

	if IsPromptSuggestionEnabled() {
		t.Error("expected false when CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION=0")
	}
}

func TestIsPromptSuggestionEnabled_FeatureFlag(t *testing.T) {
	os.Setenv("CLAUDE_FEATURE_"+featureflag.FlagPromptSuggestion, "1")
	defer os.Unsetenv("CLAUDE_FEATURE_" + featureflag.FlagPromptSuggestion)

	if !IsPromptSuggestionEnabled() {
		t.Error("expected true when CLAUDE_FEATURE_PROMPT_SUGGESTION=1")
	}
}

func TestIsPromptSuggestionEnabled_NonInteractive(t *testing.T) {
	os.Unsetenv("CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION")
	os.Setenv("CLAUDE_NON_INTERACTIVE", "1")
	defer os.Unsetenv("CLAUDE_NON_INTERACTIVE")

	if IsPromptSuggestionEnabled() {
		t.Error("expected false when CLAUDE_NON_INTERACTIVE is set")
	}
}

func TestIsPromptSuggestionEnabled_Teammate(t *testing.T) {
	os.Unsetenv("CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION")
	os.Unsetenv("CLAUDE_NON_INTERACTIVE")
	os.Setenv("CLAUDE_AGENT_ID", "agent-1")
	defer os.Unsetenv("CLAUDE_AGENT_ID")

	if IsPromptSuggestionEnabled() {
		t.Error("expected false when CLAUDE_AGENT_ID is set")
	}
}
