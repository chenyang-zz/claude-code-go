package prompts

import (
	"context"
	"strings"
	"testing"
)

func TestGlobPromptSection_Name(t *testing.T) {
	s := GlobPromptSection{}
	if got := s.Name(); got != "glob_prompt" {
		t.Errorf("Name() = %q, want %q", got, "glob_prompt")
	}
}

func TestGlobPromptSection_IsVolatile(t *testing.T) {
	s := GlobPromptSection{}
	if s.IsVolatile() {
		t.Error("IsVolatile() = true, want false")
	}
}

func TestGlobPromptSection_Compute(t *testing.T) {
	s := GlobPromptSection{}
	content, err := s.Compute(context.Background())
	if err != nil {
		t.Fatalf("Compute() error = %v", err)
	}
	if strings.TrimSpace(content) == "" {
		t.Error("Compute() returned empty content")
	}

	keyPhrases := []string{
		"Glob Tool",
		"glob patterns",
		"modification time",
		"Agent tool",
	}
	for _, phrase := range keyPhrases {
		if !strings.Contains(content, phrase) {
			t.Errorf("Compute() missing expected phrase: %q", phrase)
		}
	}
}
