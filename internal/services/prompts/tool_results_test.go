package prompts

import (
	"context"
	"testing"
)

// TestToolResultsReminderSection_Compute verifies the reminder text is stable
// and ready for inclusion in the dynamic prompt.
func TestToolResultsReminderSection_Compute(t *testing.T) {
	section := ToolResultsReminderSection{}

	got, err := section.Compute(context.Background())
	if err != nil {
		t.Fatalf("Compute() error = %v", err)
	}

	want := "When working with tool results, write down any important information you might need later in your response, as the original tool result may be cleared later."
	if got != want {
		t.Fatalf("Compute() = %q, want %q", got, want)
	}
}
