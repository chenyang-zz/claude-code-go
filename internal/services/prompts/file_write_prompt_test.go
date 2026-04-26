package prompts

import (
	"context"
	"strings"
	"testing"
)

func TestFileWritePromptSection_Name(t *testing.T) {
	s := FileWritePromptSection{}
	if got := s.Name(); got != "file_write_prompt" {
		t.Errorf("Name() = %q, want %q", got, "file_write_prompt")
	}
}

func TestFileWritePromptSection_IsVolatile(t *testing.T) {
	s := FileWritePromptSection{}
	if s.IsVolatile() {
		t.Error("IsVolatile() = true, want false")
	}
}

func TestFileWritePromptSection_Compute(t *testing.T) {
	s := FileWritePromptSection{}
	content, err := s.Compute(context.Background())
	if err != nil {
		t.Fatalf("Compute() error = %v", err)
	}
	if strings.TrimSpace(content) == "" {
		t.Error("Compute() returned empty content")
	}

	keyPhrases := []string{
		"Write Tool",
		"overwri", // "overwrite" or "overwrites"
		"Read tool first",
		"Edit tool",
		"NEVER create documentation files",
		"Only use emojis",
	}
	for _, phrase := range keyPhrases {
		if !strings.Contains(content, phrase) {
			t.Errorf("Compute() missing expected phrase: %q", phrase)
		}
	}
}
