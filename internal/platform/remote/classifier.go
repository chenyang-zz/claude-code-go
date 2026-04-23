package remote

import (
	"encoding/json"
	"fmt"
)

// MessageCategory classifies the logical kind of one remote event payload.
type MessageCategory string

const (
	// CategorySDKMessage indicates a standard SDK message (assistant, user,
	// stream_event, result, system, tool_progress, etc.).
	CategorySDKMessage MessageCategory = "sdk_message"
	// CategoryControlRequest indicates a control_request from CCR.
	CategoryControlRequest MessageCategory = "control_request"
	// CategoryControlResponse indicates a control_response (ack) from CCR.
	CategoryControlResponse MessageCategory = "control_response"
	// CategoryControlCancelRequest indicates a control_cancel_request from CCR.
	CategoryControlCancelRequest MessageCategory = "control_cancel_request"
)

// ClassifyEvent inspects the raw JSON payload and returns its message category.
// Any payload with a "type" field that is not one of the control types is
// treated as an SDK message. This matches the TS isSDKMessage guard.
func ClassifyEvent(data []byte) (MessageCategory, error) {
	var envelope struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return "", fmt.Errorf("unmarshal event type: %w", err)
	}

	switch envelope.Type {
	case "control_request":
		return CategoryControlRequest, nil
	case "control_response":
		return CategoryControlResponse, nil
	case "control_cancel_request":
		return CategoryControlCancelRequest, nil
	default:
		return CategorySDKMessage, nil
	}
}
