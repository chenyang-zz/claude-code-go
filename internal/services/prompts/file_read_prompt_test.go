package prompts

import (
	"context"
	"strings"
	"testing"
)

func TestFileReadPromptSection_Name(t *testing.T) {
	s := FileReadPromptSection{}
	if got := s.Name(); got != "file_read_prompt" {
		t.Errorf("Name() = %q, want %q", got, "file_read_prompt")
	}
}

func TestFileReadPromptSection_IsVolatile(t *testing.T) {
	s := FileReadPromptSection{}
	if s.IsVolatile() {
		t.Error("IsVolatile() = true, want false")
	}
}

func TestFileReadPromptSection_Compute(t *testing.T) {
	s := FileReadPromptSection{}
	content, err := s.Compute(context.Background())
	if err != nil {
		t.Fatalf("Compute() error = %v", err)
	}
	if strings.TrimSpace(content) == "" {
		t.Error("Compute() returned empty content")
	}

	// Key phrases from TS side prompt.ts
	keyPhrases := []string{
		"Read Tool",
		"file_path parameter must be an absolute path",
		"cat -n format",
		"line numbers starting at 1",
		"read images",
		"read PDF files",
		"Jupyter notebooks",
		"only read files, not directories",
		"screenshots",
	}
	for _, phrase := range keyPhrases {
		if !strings.Contains(content, phrase) {
			t.Errorf("Compute() missing expected phrase: %q", phrase)
		}
	}
}
