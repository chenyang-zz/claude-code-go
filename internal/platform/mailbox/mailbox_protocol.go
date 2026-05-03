package mailbox

import (
	"encoding/json"
	"time"
)

// ────────────────────────────────────────────────────────────────────────────
// Idle Notification
// ────────────────────────────────────────────────────────────────────────────

// IdleNotificationMessage is sent when a teammate becomes idle (via Stop hook).
type IdleNotificationMessage struct {
	Type        string `json:"type"` // always "idle_notification"
	From        string `json:"from"`
	Timestamp   string `json:"timestamp"`
	IdleReason  string `json:"idleReason,omitempty"`  // "available", "interrupted", or "failed"
	Summary     string `json:"summary,omitempty"`     // brief summary of last DM sent this turn
	CompletedTaskID   string `json:"completedTaskId,omitempty"`
	CompletedStatus   string `json:"completedStatus,omitempty"`   // "resolved", "blocked", or "failed"
	FailureReason     string `json:"failureReason,omitempty"`
}

// CreateIdleNotification creates an idle notification message.
func CreateIdleNotification(agentID string, idleReason string, summary string, completedTaskID string, completedStatus string, failureReason string) IdleNotificationMessage {
	return IdleNotificationMessage{
		Type:            "idle_notification",
		From:            agentID,
		Timestamp:       time.Now().Format(time.RFC3339),
		IdleReason:      idleReason,
		Summary:         summary,
		CompletedTaskID: completedTaskID,
		CompletedStatus: completedStatus,
		FailureReason:   failureReason,
	}
}

// IsIdleNotification checks if a message text contains an idle notification.
func IsIdleNotification(messageText string) (*IdleNotificationMessage, bool) {
	var msg IdleNotificationMessage
	if err := json.Unmarshal([]byte(messageText), &msg); err != nil {
		return nil, false
	}
	if msg.Type != "idle_notification" {
		return nil, false
	}
	return &msg, true
}

// ────────────────────────────────────────────────────────────────────────────
// Permission Request / Response
// ────────────────────────────────────────────────────────────────────────────

// PermissionRequestMessage is sent from worker to leader via mailbox to
// request tool execution approval. Field names align with SDK can_use_tool.
type PermissionRequestMessage struct {
	Type                  string                   `json:"type"` // always "permission_request"
	RequestID             string                   `json:"request_id"`
	AgentID               string                   `json:"agent_id"`
	ToolName              string                   `json:"tool_name"`
	ToolUseID             string                   `json:"tool_use_id"`
	Description           string                   `json:"description"`
	Input                 map[string]interface{}   `json:"input"`
	PermissionSuggestions []interface{}            `json:"permission_suggestions"`
}

// CreatePermissionRequestMessage creates a permission request message.
func CreatePermissionRequestMessage(requestID, agentID, toolName, toolUseID, description string, input map[string]interface{}, suggestions []interface{}) PermissionRequestMessage {
	if suggestions == nil {
		suggestions = []interface{}{}
	}
	return PermissionRequestMessage{
		Type:                  "permission_request",
		RequestID:             requestID,
		AgentID:               agentID,
		ToolName:              toolName,
		ToolUseID:             toolUseID,
		Description:           description,
		Input:                 input,
		PermissionSuggestions: suggestions,
	}
}

// IsPermissionRequest checks if a message text contains a permission request.
func IsPermissionRequest(messageText string) (*PermissionRequestMessage, bool) {
	var msg PermissionRequestMessage
	if err := json.Unmarshal([]byte(messageText), &msg); err != nil {
		return nil, false
	}
	if msg.Type != "permission_request" {
		return nil, false
	}
	return &msg, true
}

// PermissionResponseMessage is sent from leader to worker via mailbox in
// response to a permission request. Shape mirrors SDK ControlResponse.
type PermissionResponseMessage struct {
	Type      string `json:"type"` // always "permission_response"
	RequestID string `json:"request_id"`
	Subtype   string `json:"subtype"`           // "success" or "error"
	Response  *PermissionResponsePayload `json:"response,omitempty"`
	Error     string `json:"error,omitempty"`
}

// PermissionResponsePayload carries the successful permission response data.
type PermissionResponsePayload struct {
	UpdatedInput      map[string]interface{} `json:"updated_input,omitempty"`
	PermissionUpdates []interface{}           `json:"permission_updates,omitempty"`
}

// CreatePermissionResponseSuccess creates a successful permission response.
func CreatePermissionResponseSuccess(requestID string, updatedInput map[string]interface{}, permissionUpdates []interface{}) PermissionResponseMessage {
	return PermissionResponseMessage{
		Type:      "permission_response",
		RequestID: requestID,
		Subtype:   "success",
		Response: &PermissionResponsePayload{
			UpdatedInput:      updatedInput,
			PermissionUpdates: permissionUpdates,
		},
	}
}

// CreatePermissionResponseError creates an error permission response.
func CreatePermissionResponseError(requestID, errStr string) PermissionResponseMessage {
	return PermissionResponseMessage{
		Type:      "permission_response",
		RequestID: requestID,
		Subtype:   "error",
		Error:     errStr,
	}
}

// IsPermissionResponse checks if a message text contains a permission response.
func IsPermissionResponse(messageText string) (*PermissionResponseMessage, bool) {
	var msg PermissionResponseMessage
	if err := json.Unmarshal([]byte(messageText), &msg); err != nil {
		return nil, false
	}
	if msg.Type != "permission_response" {
		return nil, false
	}
	return &msg, true
}

// ────────────────────────────────────────────────────────────────────────────
// Sandbox Permission Request / Response
// ────────────────────────────────────────────────────────────────────────────

// SandboxPermissionRequestMessage is sent from worker to leader when sandbox
// runtime detects network access to a non-allowed host.
type SandboxPermissionRequestMessage struct {
	Type        string                    `json:"type"` // always "sandbox_permission_request"
	RequestID   string                    `json:"requestId"`
	WorkerID    string                    `json:"workerId"`
	WorkerName  string                    `json:"workerName"`
	WorkerColor string                    `json:"workerColor,omitempty"`
	HostPattern SandboxHostPattern        `json:"hostPattern"`
	CreatedAt   int64                     `json:"createdAt"`
}

// SandboxHostPattern describes the host requesting network access.
type SandboxHostPattern struct {
	Host string `json:"host"`
}

// CreateSandboxPermissionRequestMessage creates a sandbox permission request.
func CreateSandboxPermissionRequestMessage(requestID, workerID, workerName, workerColor, host string) SandboxPermissionRequestMessage {
	return SandboxPermissionRequestMessage{
		Type:        "sandbox_permission_request",
		RequestID:   requestID,
		WorkerID:    workerID,
		WorkerName:  workerName,
		WorkerColor: workerColor,
		HostPattern: SandboxHostPattern{Host: host},
		CreatedAt:   time.Now().UnixMilli(),
	}
}

// IsSandboxPermissionRequest checks if a message text contains a sandbox
// permission request.
func IsSandboxPermissionRequest(messageText string) (*SandboxPermissionRequestMessage, bool) {
	var msg SandboxPermissionRequestMessage
	if err := json.Unmarshal([]byte(messageText), &msg); err != nil {
		return nil, false
	}
	if msg.Type != "sandbox_permission_request" {
		return nil, false
	}
	return &msg, true
}

// SandboxPermissionResponseMessage is sent from leader to worker in response
// to a sandbox permission request.
type SandboxPermissionResponseMessage struct {
	Type      string `json:"type"` // always "sandbox_permission_response"
	RequestID string `json:"requestId"`
	Host      string `json:"host"`
	Allow     bool   `json:"allow"`
	Timestamp string `json:"timestamp"`
}

// CreateSandboxPermissionResponseMessage creates a sandbox permission response.
func CreateSandboxPermissionResponseMessage(requestID, host string, allow bool) SandboxPermissionResponseMessage {
	return SandboxPermissionResponseMessage{
		Type:      "sandbox_permission_response",
		RequestID: requestID,
		Host:      host,
		Allow:     allow,
		Timestamp: time.Now().Format(time.RFC3339),
	}
}

// IsSandboxPermissionResponse checks if a message text contains a sandbox
// permission response.
func IsSandboxPermissionResponse(messageText string) (*SandboxPermissionResponseMessage, bool) {
	var msg SandboxPermissionResponseMessage
	if err := json.Unmarshal([]byte(messageText), &msg); err != nil {
		return nil, false
	}
	if msg.Type != "sandbox_permission_response" {
		return nil, false
	}
	return &msg, true
}

// ────────────────────────────────────────────────────────────────────────────
// Plan Approval Request / Response
// ────────────────────────────────────────────────────────────────────────────

// PlanApprovalRequestMessage is sent when a teammate requests plan approval
// from the team leader.
type PlanApprovalRequestMessage struct {
	Type          string `json:"type"` // always "plan_approval_request"
	From          string `json:"from"`
	Timestamp     string `json:"timestamp"`
	PlanFilePath  string `json:"planFilePath"`
	PlanContent   string `json:"planContent"`
	RequestID     string `json:"requestId"`
}

// IsPlanApprovalRequest checks if a message text contains a plan approval request.
func IsPlanApprovalRequest(messageText string) (*PlanApprovalRequestMessage, bool) {
	var msg PlanApprovalRequestMessage
	if err := json.Unmarshal([]byte(messageText), &msg); err != nil {
		return nil, false
	}
	if msg.Type != "plan_approval_request" {
		return nil, false
	}
	return &msg, true
}

// PlanApprovalResponseMessage is sent by the team leader in response to a
// plan approval request.
type PlanApprovalResponseMessage struct {
	Type           string `json:"type"` // always "plan_approval_response"
	RequestID      string `json:"requestId"`
	Approved       bool   `json:"approved"`
	Feedback       string `json:"feedback,omitempty"`
	Timestamp      string `json:"timestamp"`
	PermissionMode string `json:"permissionMode,omitempty"`
}

// IsPlanApprovalResponse checks if a message text contains a plan approval response.
func IsPlanApprovalResponse(messageText string) (*PlanApprovalResponseMessage, bool) {
	var msg PlanApprovalResponseMessage
	if err := json.Unmarshal([]byte(messageText), &msg); err != nil {
		return nil, false
	}
	if msg.Type != "plan_approval_response" {
		return nil, false
	}
	return &msg, true
}

// ────────────────────────────────────────────────────────────────────────────
// Shutdown Request / Approved / Rejected
// ────────────────────────────────────────────────────────────────────────────

// ShutdownRequestMessage is sent from leader to teammate via mailbox.
type ShutdownRequestMessage struct {
	Type      string `json:"type"` // always "shutdown_request"
	RequestID string `json:"requestId"`
	From      string `json:"from"`
	Reason    string `json:"reason,omitempty"`
	Timestamp string `json:"timestamp"`
}

// CreateShutdownRequestMessage creates a shutdown request message to send
// to a teammate.
func CreateShutdownRequestMessage(requestID, from, reason string) ShutdownRequestMessage {
	return ShutdownRequestMessage{
		Type:      "shutdown_request",
		RequestID: requestID,
		From:      from,
		Reason:    reason,
		Timestamp: time.Now().Format(time.RFC3339),
	}
}

// IsShutdownRequest checks if a message text contains a shutdown request.
func IsShutdownRequest(messageText string) (*ShutdownRequestMessage, bool) {
	var msg ShutdownRequestMessage
	if err := json.Unmarshal([]byte(messageText), &msg); err != nil {
		return nil, false
	}
	if msg.Type != "shutdown_request" {
		return nil, false
	}
	return &msg, true
}

// ShutdownApprovedMessage is sent from teammate to leader via mailbox to
// confirm shutdown.
type ShutdownApprovedMessage struct {
	Type        string `json:"type"` // always "shutdown_approved"
	RequestID   string `json:"requestId"`
	From        string `json:"from"`
	Timestamp   string `json:"timestamp"`
	PaneID      string `json:"paneId,omitempty"`
	BackendType string `json:"backendType,omitempty"`
}

// CreateShutdownApprovedMessage creates a shutdown approved message.
func CreateShutdownApprovedMessage(requestID, from, paneID, backendType string) ShutdownApprovedMessage {
	return ShutdownApprovedMessage{
		Type:        "shutdown_approved",
		RequestID:   requestID,
		From:        from,
		Timestamp:   time.Now().Format(time.RFC3339),
		PaneID:      paneID,
		BackendType: backendType,
	}
}

// IsShutdownApproved checks if a message text contains a shutdown approved message.
func IsShutdownApproved(messageText string) (*ShutdownApprovedMessage, bool) {
	var msg ShutdownApprovedMessage
	if err := json.Unmarshal([]byte(messageText), &msg); err != nil {
		return nil, false
	}
	if msg.Type != "shutdown_approved" {
		return nil, false
	}
	return &msg, true
}

// ShutdownRejectedMessage is sent from teammate to leader via mailbox to
// reject a shutdown request.
type ShutdownRejectedMessage struct {
	Type      string `json:"type"` // always "shutdown_rejected"
	RequestID string `json:"requestId"`
	From      string `json:"from"`
	Reason    string `json:"reason"`
	Timestamp string `json:"timestamp"`
}

// CreateShutdownRejectedMessage creates a shutdown rejected message.
func CreateShutdownRejectedMessage(requestID, from, reason string) ShutdownRejectedMessage {
	return ShutdownRejectedMessage{
		Type:      "shutdown_rejected",
		RequestID: requestID,
		From:      from,
		Reason:    reason,
		Timestamp: time.Now().Format(time.RFC3339),
	}
}

// IsShutdownRejected checks if a message text contains a shutdown rejected message.
func IsShutdownRejected(messageText string) (*ShutdownRejectedMessage, bool) {
	var msg ShutdownRejectedMessage
	if err := json.Unmarshal([]byte(messageText), &msg); err != nil {
		return nil, false
	}
	if msg.Type != "shutdown_rejected" {
		return nil, false
	}
	return &msg, true
}

// ────────────────────────────────────────────────────────────────────────────
// Task Assignment
// ────────────────────────────────────────────────────────────────────────────

// TaskAssignmentMessage is sent when a task is assigned to a teammate.
type TaskAssignmentMessage struct {
	Type        string `json:"type"` // always "task_assignment"
	TaskID      string `json:"taskId"`
	Subject     string `json:"subject"`
	Description string `json:"description"`
	AssignedBy  string `json:"assignedBy"`
	Timestamp   string `json:"timestamp"`
}

// IsTaskAssignment checks if a message text contains a task assignment.
func IsTaskAssignment(messageText string) (*TaskAssignmentMessage, bool) {
	var msg TaskAssignmentMessage
	if err := json.Unmarshal([]byte(messageText), &msg); err != nil {
		return nil, false
	}
	if msg.Type != "task_assignment" {
		return nil, false
	}
	return &msg, true
}

// ────────────────────────────────────────────────────────────────────────────
// Team Permission Update
// ────────────────────────────────────────────────────────────────────────────

// TeamPermissionUpdateMessage is sent from leader to teammates via mailbox
// to broadcast a permission update that applies to all teammates.
type TeamPermissionUpdateMessage struct {
	Type             string                          `json:"type"` // always "team_permission_update"
	PermissionUpdate TeamPermissionUpdatePayload      `json:"permissionUpdate"`
	DirectoryPath    string                          `json:"directoryPath"`
	ToolName         string                          `json:"toolName"`
}

// TeamPermissionUpdatePayload describes the permission change.
type TeamPermissionUpdatePayload struct {
	Type        string                            `json:"type"` // always "addRules"
	Rules       []TeamPermissionUpdateRule         `json:"rules"`
	Behavior    string                            `json:"behavior"` // "allow", "deny", or "ask"
	Destination string                            `json:"destination"` // "session"
}

// TeamPermissionUpdateRule is a single permission rule to add.
type TeamPermissionUpdateRule struct {
	ToolName    string `json:"toolName"`
	RuleContent string `json:"ruleContent,omitempty"`
}

// IsTeamPermissionUpdate checks if a message text contains a team permission update.
func IsTeamPermissionUpdate(messageText string) (*TeamPermissionUpdateMessage, bool) {
	var msg TeamPermissionUpdateMessage
	if err := json.Unmarshal([]byte(messageText), &msg); err != nil {
		return nil, false
	}
	if msg.Type != "team_permission_update" {
		return nil, false
	}
	return &msg, true
}

// ────────────────────────────────────────────────────────────────────────────
// Mode Set Request
// ────────────────────────────────────────────────────────────────────────────

// ModeSetRequestMessage is sent from leader to teammate via mailbox to change
// the teammate's permission mode.
type ModeSetRequestMessage struct {
	Type string `json:"type"` // always "mode_set_request"
	Mode string `json:"mode"`
	From string `json:"from"`
}

// CreateModeSetRequestMessage creates a mode set request message.
func CreateModeSetRequestMessage(mode, from string) ModeSetRequestMessage {
	return ModeSetRequestMessage{
		Type: "mode_set_request",
		Mode: mode,
		From: from,
	}
}

// IsModeSetRequest checks if a message text contains a mode set request.
func IsModeSetRequest(messageText string) (*ModeSetRequestMessage, bool) {
	var msg ModeSetRequestMessage
	if err := json.Unmarshal([]byte(messageText), &msg); err != nil {
		return nil, false
	}
	if msg.Type != "mode_set_request" {
		return nil, false
	}
	return &msg, true
}

// ────────────────────────────────────────────────────────────────────────────
// Structured Protocol Message Router
// ────────────────────────────────────────────────────────────────────────────

// structuredTypes is the set of protocol message types that must be routed
// by inbox pollers rather than consumed as raw LLM context.
var structuredTypes = map[string]bool{
	"permission_request":            true,
	"permission_response":           true,
	"sandbox_permission_request":    true,
	"sandbox_permission_response":   true,
	"shutdown_request":              true,
	"shutdown_approved":             true,
	"team_permission_update":        true,
	"mode_set_request":              true,
	"plan_approval_request":         true,
	"plan_approval_response":        true,
}

// typeProbe is a minimal struct used to extract the "type" field from a JSON
// payload without unmarshalling the full message.
type typeProbe struct {
	Type string `json:"type"`
}

// IsStructuredProtocolMessage checks whether a message text is a structured
// protocol message that should be routed by useInboxPoller rather than
// consumed as raw LLM context. These message types have specific handlers
// that route them to the correct queues (workerPermissions,
// workerSandboxPermissions, etc.).
func IsStructuredProtocolMessage(messageText string) bool {
	var probe typeProbe
	if err := json.Unmarshal([]byte(messageText), &probe); err != nil {
		return false
	}
	return structuredTypes[probe.Type]
}
