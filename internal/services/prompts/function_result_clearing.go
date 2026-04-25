package prompts

import "context"

// FunctionResultClearingSection warns that older tool outputs may be removed
// from context to keep the prompt within budget.
type FunctionResultClearingSection struct{}

// Name returns the section identifier.
func (s FunctionResultClearingSection) Name() string { return "frc" }

// IsVolatile reports whether this section must be recomputed every turn.
func (s FunctionResultClearingSection) IsVolatile() bool { return false }

// Compute generates the function result clearing reminder.
func (s FunctionResultClearingSection) Compute(ctx context.Context) (string, error) {
	return `# Function Result Clearing

Older tool results may be cleared from context to free up space. Keep any important findings in mind or in your response before you move on.`, nil
}
