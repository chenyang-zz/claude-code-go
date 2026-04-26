package prompts

import (
	"context"
	"strings"
	"testing"
)

func TestGrepPromptSection_Name(t *testing.T) {
	s := GrepPromptSection{}
	if got := s.Name(); got != "grep_prompt" {
		t.Errorf("Name() = %q, want %q", got, "grep_prompt")
	}
}

func TestGrepPromptSection_IsVolatile(t *testing.T) {
	s := GrepPromptSection{}
	if s.IsVolatile() {
		t.Error("IsVolatile() = true, want false")
	}
}

func TestGrepPromptSection_Compute(t *testing.T) {
	s := GrepPromptSection{}
	content, err := s.Compute(context.Background())
	if err != nil {
		t.Fatalf("Compute() error = %v", err)
	}
	if strings.TrimSpace(content) == "" {
		t.Error("Compute() returned empty content")
	}

	keyPhrases := []string{
		"Grep Tool",
		"ripgrep",
		"NEVER invoke",
		"regex syntax",
		"glob parameter",
		"Output modes",
		"Agent tool",
		"Multiline matching",
	}
	for _, phrase := range keyPhrases {
		if !strings.Contains(content, phrase) {
			t.Errorf("Compute() missing expected phrase: %q", phrase)
		}
	}
}
