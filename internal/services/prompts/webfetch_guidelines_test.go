package prompts

import (
	"context"
	"strings"
	"testing"
)

// TestWebFetchGuidelinesSection_Name verifies the section identifier.
func TestWebFetchGuidelinesSection_Name(t *testing.T) {
	section := WebFetchGuidelinesSection{}
	if got := section.Name(); got != "webfetch_guidelines" {
		t.Errorf("Name() = %q, want %q", got, "webfetch_guidelines")
	}
}

// TestWebFetchGuidelinesSection_IsVolatile verifies the section is non-volatile.
func TestWebFetchGuidelinesSection_IsVolatile(t *testing.T) {
	section := WebFetchGuidelinesSection{}
	if got := section.IsVolatile(); got {
		t.Error("IsVolatile() = true, want false")
	}
}

// TestWebFetchGuidelinesSection_Compute verifies the guidelines contain key guidance topics.
func TestWebFetchGuidelinesSection_Compute(t *testing.T) {
	section := WebFetchGuidelinesSection{}
	got, err := section.Compute(context.Background())
	if err != nil {
		t.Fatalf("Compute() error = %v", err)
	}
	if got == "" {
		t.Fatal("Compute() returned empty string")
	}

	checks := []string{
		"WebFetch",
		"When to use",
		"When NOT to use",
		"URL requirements",
		"Permission behavior",
		"Output format",
		"Caching",
		"Redirects",
	}
	for _, want := range checks {
		if !strings.Contains(got, want) {
			t.Errorf("Compute() missing expected content: %q", want)
		}
	}
}
