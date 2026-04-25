package prompts

import (
	"context"
	"testing"
)

// TestFunctionResultClearingSection_Compute verifies the clearing reminder is stable.
func TestFunctionResultClearingSection_Compute(t *testing.T) {
	section := FunctionResultClearingSection{}

	got, err := section.Compute(context.Background())
	if err != nil {
		t.Fatalf("Compute() error = %v", err)
	}

	want := `# Function Result Clearing

Older tool results may be cleared from context to free up space. Keep any important findings in mind or in your response before you move on.`
	if got != want {
		t.Fatalf("Compute() = %q, want %q", got, want)
	}
}
