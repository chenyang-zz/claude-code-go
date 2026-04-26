package prompts

import (
	"context"
	"strings"
	"testing"
)

func TestFileEditPromptSection_Name(t *testing.T) {
	s := FileEditPromptSection{}
	if got := s.Name(); got != "file_edit_prompt" {
		t.Errorf("Name() = %q, want %q", got, "file_edit_prompt")
	}
}

func TestFileEditPromptSection_IsVolatile(t *testing.T) {
	s := FileEditPromptSection{}
	if s.IsVolatile() {
		t.Error("IsVolatile() = true, want false")
	}
}

func TestFileEditPromptSection_Compute(t *testing.T) {
	s := FileEditPromptSection{}
	content, err := s.Compute(context.Background())
	if err != nil {
		t.Fatalf("Compute() error = %v", err)
	}
	if strings.TrimSpace(content) == "" {
		t.Error("Compute() returned empty content")
	}

	keyPhrases := []string{
		"Edit Tool",
		"Read tool at least once",
		"preserve the exact indentation",
		"line number prefix",
		"old_string",
		"replace_all",
		"FAIL",
	}
	for _, phrase := range keyPhrases {
		if !strings.Contains(content, phrase) {
			t.Errorf("Compute() missing expected phrase: %q", phrase)
		}
	}
}
