package prompts

import "context"

// SendMessagePromptSection provides usage guidance for the SendMessage tool.
type SendMessagePromptSection struct{}

// Name returns the section identifier.
func (s SendMessagePromptSection) Name() string { return "send_message_prompt" }

// IsVolatile reports whether this section must be recomputed every turn.
func (s SendMessagePromptSection) IsVolatile() bool { return false }

// Compute generates the SendMessage tool usage guidance.
func (s SendMessagePromptSection) Compute(ctx context.Context) (string, error) {
	return "# SendMessage\n\n" +
		"Send a message to another agent.\n\n" +
		"```json\n" +
		`{"to": "researcher", "summary": "assign task 1", "message": "start on task #1"}` + "\n" +
		"```\n\n" +
		"| to | |\n" +
		"|---|---|\n" +
		"| \"researcher\" | Teammate by name |\n" +
		"| \"*\" | Broadcast to all teammates — expensive (linear in team size), use only when everyone genuinely needs it |\n\n" +
		"Your plain text output is NOT visible to other agents — to communicate, you MUST call this tool. Messages from teammates are delivered automatically; you don't check an inbox. Refer to teammates by name, never by UUID. When relaying, don't quote the original — it's already rendered to the user.\n\n" +
		"## Protocol responses (legacy)\n\n" +
		"If you receive a JSON message with type: \"shutdown_request\" or type: \"plan_approval_request\", respond with the matching _response type — echo the request_id, set approve true/false:\n\n" +
		"```json\n" +
		`{"to": "team-lead", "message": {"type": "shutdown_response", "request_id": "...", "approve": true}}` + "\n" +
		`{"to": "researcher", "message": {"type": "plan_approval_response", "request_id": "...", "approve": false, "feedback": "add error handling"}}` + "\n" +
		"```\n\n" +
		"Approving shutdown terminates your process. Rejecting plan sends the teammate back to revise. Don't originate shutdown_request unless asked. Don't send structured JSON status messages — use TaskUpdate.", nil
}
