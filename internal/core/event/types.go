package event

import (
	"github.com/sheepzhao/claude-code-go/internal/core/conversation"
	"github.com/sheepzhao/claude-code-go/internal/core/model"
)

// MessageDeltaPayload carries one assistant text chunk rendered to the caller.
type MessageDeltaPayload struct {
	Text string `json:"text"`
}

// ThinkingPayload carries one complete assistant thinking block rendered to the caller.
type ThinkingPayload struct {
	Thinking  string `json:"thinking"`
	Signature string `json:"signature,omitempty"`
}

// ToolCallPayload describes one tool_use event surfaced to the runtime caller.
type ToolCallPayload struct {
	ID    string         `json:"id"`
	Name  string         `json:"name"`
	Input map[string]any `json:"input"`
}

// ToolResultPayload describes one completed tool execution inside the runtime loop.
type ToolResultPayload struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	Output            string `json:"output"`
	AdditionalContext string `json:"additional_context,omitempty"`
	IsError           bool   `json:"is_error"`
}

// ApprovalPayload describes one runtime approval request emitted before a guarded tool operation can continue.
type ApprovalPayload struct {
	CallID   string `json:"call_id"`
	ToolName string `json:"tool_name"`
	Path     string `json:"path"`
	Action   string `json:"action"`
	Message  string `json:"message,omitempty"`
}

// ErrorPayload carries one runtime or provider error message.
type ErrorPayload struct {
	Message string `json:"message"`
}

// ConversationDonePayload carries the final normalized history produced by one runtime turn.
type ConversationDonePayload struct {
	History conversation.History `json:"history"`
	Usage   model.Usage          `json:"usage"`
	// StopReason carries the provider/runtime stop reason for the final turn.
	StopReason string `json:"stop_reason,omitempty"`
}

// UsagePayload carries per-turn and cumulative token usage metrics.
type UsagePayload struct {
	TurnUsage       model.Usage `json:"turn_usage"`
	CumulativeUsage model.Usage `json:"cumulative_usage"`
	StopReason      string      `json:"stop_reason"`
}

// RetryAttemptedPayload carries information about one retry attempt.
type RetryAttemptedPayload struct {
	Attempt     int    `json:"attempt"`
	MaxAttempts int    `json:"max_attempts"`
	BackoffMs   int64  `json:"backoff_ms"`
	Error       string `json:"error"`
	ErrorKind   string `json:"error_kind,omitempty"`
}

// ModelFallbackPayload carries information about a model fallback switch.
type ModelFallbackPayload struct {
	OriginalModel string `json:"original_model"`
	FallbackModel string `json:"fallback_model"`
}

// CompactDonePayload carries information about a completed auto-compaction.
type CompactDonePayload struct {
	PreTokenCount  int `json:"pre_token_count"`
	PostTokenCount int `json:"post_token_count"`
}

// ProgressPayload carries one incremental progress update emitted by a running tool.
type ProgressPayload struct {
	// ToolUseID identifies the tool invocation this progress belongs to.
	ToolUseID string `json:"tool_use_id"`
	// ParentToolUseID links this progress to a parent tool call when nested.
	ParentToolUseID string `json:"parent_tool_use_id,omitempty"`
	// Data holds the type-specific progress details (e.g. BashProgressData).
	Data any `json:"data,omitempty"`
}
