package brief

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
)

const Name = "SendUserMessage"

// legacyAlias exposes the legacy name "Brief" for backward compatibility.
const legacyAlias = "Brief"

// Tool implements the SendUserMessage tool, which allows the model to send
// a message directly to the user. It is the primary visible output channel.
type Tool struct{}

// NewTool creates a new SendUserMessage tool.
func NewTool() *Tool {
	return &Tool{}
}

func (t *Tool) Name() string {
	if t == nil {
		return Name
	}
	return Name
}

func (t *Tool) Description() string {
	return "Send a message to the user"
}

func (t *Tool) Aliases() []string {
	if t == nil {
		return nil
	}
	return []string{legacyAlias}
}

// briefInput defines the input schema for the SendUserMessage tool.
type briefInput struct {
	// Message is the markdown-formatted message to show the user.
	Message string `json:"message"`
	// Attachments are optional file paths to attach.
	Attachments []string `json:"attachments,omitempty"`
	// Status labels intent: "normal" when replying, "proactive" when initiating.
	Status string `json:"status,omitempty"`
}

// briefOutput defines the output returned to the model.
type briefOutput struct {
	// Message echoes the sent message.
	Message string `json:"message"`
	// SentAt is the ISO 8601 timestamp captured at invocation.
	SentAt string `json:"sentAt"`
}

func (t *Tool) InputSchema() coretool.InputSchema {
	return coretool.InputSchema{
		Properties: map[string]coretool.FieldSchema{
			"message": {
				Type:        coretool.ValueKindString,
				Description: "The message for the user. Supports markdown formatting.",
				Required:    true,
			},
			"attachments": {
				Type:        coretool.ValueKindArray,
				Description: "Optional file paths (absolute or relative to cwd) to attach. Use for photos, screenshots, diffs, logs, or any file the user should see alongside your message.",
			},
			"status": {
				Type:        coretool.ValueKindString,
				Description: "Use 'proactive' when you're surfacing something the user hasn't asked for and needs to see now — task completion while they're away, a blocker you hit, an unsolicited status update. Use 'normal' when replying to something the user just said.",
			},
		},
	}
}

func (t *Tool) IsReadOnly() bool        { return true }
func (t *Tool) IsConcurrencySafe() bool { return true }

// Invoke returns a message delivery confirmation with the sent timestamp.
func (t *Tool) Invoke(ctx context.Context, call coretool.Call) (coretool.Result, error) {
	if t == nil {
		return coretool.Result{Error: "SendUserMessage tool: nil receiver"}, nil
	}

	input, err := coretool.DecodeInput[briefInput](t.InputSchema(), call.Input)
	if err != nil {
		return coretool.Result{Error: fmt.Sprintf("invalid input: %v", err)}, nil
	}

	if strings.TrimSpace(input.Message) == "" {
		return coretool.Result{Error: "message must not be empty"}, nil
	}

	sentAt := time.Now().Format(time.RFC3339)
	output := briefOutput{
		Message: input.Message,
		SentAt:  sentAt,
	}

	data, _ := json.Marshal(output)
	return coretool.Result{Output: string(data)}, nil
}
