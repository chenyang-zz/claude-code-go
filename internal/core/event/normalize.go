package event

import (
	"fmt"

	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	"github.com/sheepzhao/claude-code-go/pkg/sdk"
)

// ToSDKMessage normalizes an internal Event to an SDK Message.
// Events that have no SDK counterpart return nil.
func (evt Event) ToSDKMessage() sdk.Message {
	switch evt.Type {
	case TypeMessageDelta:
		return normalizeMessageDelta(evt)
	case TypeProgress:
		return normalizeProgress(evt)
	case TypeConversationDone:
		return normalizeConversationDone(evt)
	case TypeError:
		return normalizeError(evt)
	default:
		// Types with no SDK counterpart:
		// TypeToolCallStarted, TypeToolCallFinished, TypeApprovalRequired,
		// TypeUsage, TypeModelFallback, TypeRetryAttempted, TypeCompactDone.
		return nil
	}
}

func normalizeMessageDelta(evt Event) sdk.Message {
	payload, ok := evt.Payload.(MessageDeltaPayload)
	if !ok {
		return nil
	}
	return sdk.StreamEvent{
		Base: sdk.Base{Type: "stream_event"},
		Event: map[string]any{
			"type": "text_delta",
			"text": payload.Text,
		},
	}
}

func normalizeProgress(evt Event) sdk.Message {
	payload, ok := evt.Payload.(ProgressPayload)
	if !ok {
		return nil
	}

	toolName := ""
	elapsedTimeSec := 0.0
	switch data := payload.Data.(type) {
	case coretool.MCPProgressData:
		toolName = data.ToolName
		if data.ServerName != "" && data.ToolName != "" {
			toolName = fmt.Sprintf("%s__%s", data.ServerName, data.ToolName)
		}
		if data.ElapsedMs > 0 {
			elapsedTimeSec = float64(data.ElapsedMs) / 1000.0
		}
	case coretool.AgentToolProgressData:
		toolName = "Agent"
		if data.DurationMs > 0 {
			elapsedTimeSec = float64(data.DurationMs) / 1000.0
		}
	}

	var parentID *string
	if payload.ParentToolUseID != "" {
		parentID = &payload.ParentToolUseID
	}
	return sdk.ToolProgress{
		Base:            sdk.Base{Type: "tool_progress"},
		ToolUseID:       payload.ToolUseID,
		ParentToolUseID: parentID,
		ToolName:        toolName,
		ElapsedTimeSec:  elapsedTimeSec,
		Progress:        payload.Data,
	}
}

func normalizeConversationDone(evt Event) sdk.Message {
	payload, ok := evt.Payload.(ConversationDonePayload)
	if !ok {
		return nil
	}
	stopReason := payload.StopReason
	if stopReason == "" {
		stopReason = "end_turn"
	}
	return sdk.Result{
		Base:       sdk.Base{Type: "result"},
		Subtype:    "success",
		IsError:    false,
		Usage:      payload.Usage,
		StopReason: strPtr(stopReason),
	}
}

func normalizeError(evt Event) sdk.Message {
	payload, ok := evt.Payload.(ErrorPayload)
	if !ok {
		return nil
	}
	return sdk.Result{
		Base:       sdk.Base{Type: "result"},
		Subtype:    "error_during_execution",
		IsError:    true,
		Errors:     []string{payload.Message},
		StopReason: strPtr("error"),
	}
}

func strPtr(s string) *string {
	return &s
}
