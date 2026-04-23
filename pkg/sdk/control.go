package sdk

import "encoding/json"

// ControlRequest is a control protocol message sent from the CCR backend to the CLI.
type ControlRequest struct {
	Type      string          `json:"type"`
	RequestID string          `json:"request_id"`
	Request   json.RawMessage `json:"request"`
}

// ControlResponse is a control protocol message sent from the CLI back to CCR.
type ControlResponse struct {
	Type     string               `json:"type"`
	Response ControlResponseInner `json:"response"`
}

// ControlResponseInner carries either a success or error payload.
type ControlResponseInner struct {
	Subtype   string         `json:"subtype"`
	RequestID string         `json:"request_id"`
	Response  map[string]any `json:"response,omitempty"`
	Error     string         `json:"error,omitempty"`
}

// ControlCancelRequest cancels a pending control request.
type ControlCancelRequest struct {
	Type      string `json:"type"`
	RequestID string `json:"request_id"`
}

// ControlPermissionRequest is the inner payload for a "can_use_tool" control request.
type ControlPermissionRequest struct {
	Subtype               string         `json:"subtype"`
	ToolName              string         `json:"tool_name"`
	Input                 map[string]any `json:"input"`
	PermissionSuggestions []any          `json:"permission_suggestions,omitempty"`
	BlockedPath           string         `json:"blocked_path,omitempty"`
	DecisionReason        string         `json:"decision_reason,omitempty"`
	Title                 string         `json:"title,omitempty"`
	DisplayName           string         `json:"display_name,omitempty"`
	ToolUseID             string         `json:"tool_use_id"`
	AgentID               string         `json:"agent_id,omitempty"`
	Description           string         `json:"description,omitempty"`
}

// PermissionResponse is the local CLI decision for a remote permission request.
type PermissionResponse struct {
	Behavior     string
	UpdatedInput map[string]any
	Message      string
}

// ControlInterruptRequest is the inner payload for an "interrupt" control request.
type ControlInterruptRequest struct {
	Subtype string `json:"subtype"`
}
