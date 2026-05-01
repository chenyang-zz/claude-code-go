package prompts

import "context"

// RemoteTriggerPromptSection provides usage guidance for the RemoteTrigger tool.
type RemoteTriggerPromptSection struct{}

// Name returns the section identifier.
func (s RemoteTriggerPromptSection) Name() string { return "remote_trigger_prompt" }

// IsVolatile reports whether this section must be recomputed every turn.
func (s RemoteTriggerPromptSection) IsVolatile() bool { return false }

// Compute generates the RemoteTrigger tool usage guidance.
func (s RemoteTriggerPromptSection) Compute(ctx context.Context) (string, error) {
	return `# RemoteTrigger

Call the claude.ai remote-trigger API. Use this instead of curl — the OAuth token is added automatically in-process and never exposed.

Actions:
- list: GET /v1/code/triggers
- get: GET /v1/code/triggers/{trigger_id}
- create: POST /v1/code/triggers (requires body)
- update: POST /v1/code/triggers/{trigger_id} (requires body, partial update)
- run: POST /v1/code/triggers/{trigger_id}/run

The response is the raw JSON from the API.`, nil
}
