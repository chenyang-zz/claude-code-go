package prompts

import "context"

// ToolResultsReminderSection reminds the model to preserve important findings
// from tool output before the result is lost from context.
type ToolResultsReminderSection struct{}

// Name returns the section identifier.
func (s ToolResultsReminderSection) Name() string { return "summarize_tool_results" }

// IsVolatile reports whether this section must be recomputed every turn.
func (s ToolResultsReminderSection) IsVolatile() bool { return false }

// Compute generates the reminder text for tool result handling.
func (s ToolResultsReminderSection) Compute(ctx context.Context) (string, error) {
	return "When working with tool results, write down any important information you might need later in your response, as the original tool result may be cleared later.", nil
}
