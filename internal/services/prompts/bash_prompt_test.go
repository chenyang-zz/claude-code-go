package prompts

import (
	"context"
	"strings"
	"testing"
)

func TestBashPromptSection_Name(t *testing.T) {
	s := BashPromptSection{}
	if got := s.Name(); got != "bash_prompt" {
		t.Errorf("Name() = %q, want %q", got, "bash_prompt")
	}
}

func TestBashPromptSection_IsVolatile(t *testing.T) {
	s := BashPromptSection{}
	if s.IsVolatile() {
		t.Error("IsVolatile() = true, want false")
	}
}

func TestBashPromptSection_Compute(t *testing.T) {
	s := BashPromptSection{}
	content, err := s.Compute(context.Background())
	if err != nil {
		t.Fatalf("Compute() error = %v", err)
	}
	if strings.TrimSpace(content) == "" {
		t.Error("Compute() returned empty content")
	}

	keyPhrases := []string{
		"Bash Tool",
		"working directory persists",
		"shell state does not",
		"Avoid using this tool",
		"run_in_background",
		"multiple commands",
		"parallel",
		"git commands",
		"new commit",
		"destructive operations",
		"skip hooks",
		"sleep",
		"timeout",
		"120000ms",
	}
	for _, phrase := range keyPhrases {
		if !strings.Contains(content, phrase) {
			t.Errorf("Compute() missing expected phrase: %q", phrase)
		}
	}
}
