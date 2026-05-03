package mailbox

import (
	"encoding/json"
	"testing"
)

// ── IdleNotification ──────────────────────────────────────────────────────

func TestCreateIdleNotification(t *testing.T) {
	msg := CreateIdleNotification("agent-1", "available", "finished task", "task-123", "resolved", "")
	if msg.Type != "idle_notification" {
		t.Errorf("Type = %q, want idle_notification", msg.Type)
	}
	if msg.From != "agent-1" {
		t.Errorf("From = %q, want agent-1", msg.From)
	}
	if msg.IdleReason != "available" {
		t.Errorf("IdleReason = %q, want available", msg.IdleReason)
	}
	if msg.CompletedTaskID != "task-123" {
		t.Errorf("CompletedTaskID = %q, want task-123", msg.CompletedTaskID)
	}
}

func TestIsIdleNotification_Valid(t *testing.T) {
	msg := CreateIdleNotification("agent-1", "interrupted", "", "", "failed", "crashed")
	data, _ := json.Marshal(msg)
	parsed, ok := IsIdleNotification(string(data))
	if !ok {
		t.Fatal("expected idle notification")
	}
	if parsed.From != "agent-1" {
		t.Errorf("From = %q, want agent-1", parsed.From)
	}
}

func TestIsIdleNotification_NotIdle(t *testing.T) {
	_, ok := IsIdleNotification(`{"type": "other"}`)
	if ok {
		t.Error("should not match non-idle_notification type")
	}
}

func TestIsIdleNotification_InvalidJSON(t *testing.T) {
	_, ok := IsIdleNotification("not json")
	if ok {
		t.Error("should not match invalid JSON")
	}
}

// ── PermissionRequest ─────────────────────────────────────────────────────

func TestCreatePermissionRequestMessage(t *testing.T) {
	input := map[string]interface{}{"path": "/tmp/test"}
	suggestions := []interface{}{"allow-read"}
	msg := CreatePermissionRequestMessage("req-1", "agent-1", "FileReadTool", "toolu_001", "Read a file", input, suggestions)
	if msg.Type != "permission_request" {
		t.Errorf("Type = %q, want permission_request", msg.Type)
	}
	if msg.ToolName != "FileReadTool" {
		t.Errorf("ToolName = %q, want FileReadTool", msg.ToolName)
	}
	if msg.PermissionSuggestions == nil {
		t.Error("PermissionSuggestions should not be nil")
	}
}

func TestIsPermissionRequest_Valid(t *testing.T) {
	input := map[string]interface{}{"key": "value"}
	msg := CreatePermissionRequestMessage("req-2", "agent-2", "BashTool", "toolu_002", "Run cmd", input, nil)
	data, _ := json.Marshal(msg)
	parsed, ok := IsPermissionRequest(string(data))
	if !ok {
		t.Fatal("expected permission_request")
	}
	if parsed.RequestID != "req-2" {
		t.Errorf("RequestID = %q, want req-2", parsed.RequestID)
	}
}

func TestIsPermissionRequest_NotPermission(t *testing.T) {
	_, ok := IsPermissionRequest(`{"type": "idle_notification"}`)
	if ok {
		t.Error("should not match non-permission_request type")
	}
}

// ── PermissionResponse ────────────────────────────────────────────────────

func TestCreatePermissionResponseSuccess(t *testing.T) {
	updated := map[string]interface{}{"path": "/safe/path"}
	updates := []interface{}{}
	msg := CreatePermissionResponseSuccess("req-1", updated, updates)
	if msg.Subtype != "success" {
		t.Errorf("Subtype = %q, want success", msg.Subtype)
	}
	if msg.Response.UpdatedInput["path"] != "/safe/path" {
		t.Error("UpdatedInput not preserved")
	}
}

func TestCreatePermissionResponseError(t *testing.T) {
	msg := CreatePermissionResponseError("req-1", "access denied")
	if msg.Subtype != "error" {
		t.Errorf("Subtype = %q, want error", msg.Subtype)
	}
	if msg.Error != "access denied" {
		t.Errorf("Error = %q, want access denied", msg.Error)
	}
}

func TestIsPermissionResponse_Valid(t *testing.T) {
	msg := CreatePermissionResponseSuccess("req-3", nil, nil)
	data, _ := json.Marshal(msg)
	parsed, ok := IsPermissionResponse(string(data))
	if !ok {
		t.Fatal("expected permission_response")
	}
	if parsed.Subtype != "success" {
		t.Errorf("Subtype = %q, want success", parsed.Subtype)
	}
}

// ── SandboxPermission ─────────────────────────────────────────────────────

func TestCreateSandboxPermissionRequestMessage(t *testing.T) {
	msg := CreateSandboxPermissionRequestMessage("sr-1", "w1", "worker1", "blue", "api.example.com")
	if msg.Type != "sandbox_permission_request" {
		t.Errorf("Type = %q, want sandbox_permission_request", msg.Type)
	}
	if msg.HostPattern.Host != "api.example.com" {
		t.Errorf("Host = %q, want api.example.com", msg.HostPattern.Host)
	}
	if msg.CreatedAt == 0 {
		t.Error("CreatedAt should be non-zero")
	}
}

func TestIsSandboxPermissionRequest_Valid(t *testing.T) {
	msg := CreateSandboxPermissionRequestMessage("sr-1", "w1", "worker1", "", "host.com")
	data, _ := json.Marshal(msg)
	parsed, ok := IsSandboxPermissionRequest(string(data))
	if !ok {
		t.Fatal("expected sandbox_permission_request")
	}
	if parsed.RequestID != "sr-1" {
		t.Errorf("RequestID = %q, want sr-1", parsed.RequestID)
	}
}

func TestCreateSandboxPermissionResponseMessage(t *testing.T) {
	msg := CreateSandboxPermissionResponseMessage("sr-1", "api.example.com", true)
	if msg.Type != "sandbox_permission_response" {
		t.Errorf("Type = %q, want sandbox_permission_response", msg.Type)
	}
	if !msg.Allow {
		t.Error("Allow should be true")
	}
}

func TestIsSandboxPermissionResponse_Valid(t *testing.T) {
	msg := CreateSandboxPermissionResponseMessage("sr-1", "host.com", false)
	data, _ := json.Marshal(msg)
	parsed, ok := IsSandboxPermissionResponse(string(data))
	if !ok {
		t.Fatal("expected sandbox_permission_response")
	}
	if parsed.Allow {
		t.Error("Allow should be false")
	}
}

// ── ShutdownMessages ──────────────────────────────────────────────────────

func TestCreateShutdownRequestMessage(t *testing.T) {
	msg := CreateShutdownRequestMessage("sd-1", "leader", "work complete")
	if msg.Type != "shutdown_request" {
		t.Errorf("Type = %q, want shutdown_request", msg.Type)
	}
	if msg.From != "leader" {
		t.Errorf("From = %q, want leader", msg.From)
	}
	if msg.Reason != "work complete" {
		t.Errorf("Reason = %q, want work complete", msg.Reason)
	}
}

func TestIsShutdownRequest_Valid(t *testing.T) {
	msg := CreateShutdownRequestMessage("sd-1", "leader", "")
	data, _ := json.Marshal(msg)
	parsed, ok := IsShutdownRequest(string(data))
	if !ok {
		t.Fatal("expected shutdown_request")
	}
	if parsed.RequestID != "sd-1" {
		t.Errorf("RequestID = %q, want sd-1", parsed.RequestID)
	}
}

func TestCreateShutdownApprovedMessage(t *testing.T) {
	msg := CreateShutdownApprovedMessage("sd-1", "worker", "pane3", "tmux")
	if msg.Type != "shutdown_approved" {
		t.Errorf("Type = %q, want shutdown_approved", msg.Type)
	}
	if msg.PaneID != "pane3" {
		t.Errorf("PaneID = %q, want pane3", msg.PaneID)
	}
}

func TestIsShutdownApproved_Valid(t *testing.T) {
	msg := CreateShutdownApprovedMessage("sd-1", "worker", "", "")
	data, _ := json.Marshal(msg)
	parsed, ok := IsShutdownApproved(string(data))
	if !ok {
		t.Fatal("expected shutdown_approved")
	}
	if parsed.From != "worker" {
		t.Errorf("From = %q, want worker", parsed.From)
	}
}

func TestCreateShutdownRejectedMessage(t *testing.T) {
	msg := CreateShutdownRejectedMessage("sd-1", "worker", "still processing")
	if msg.Type != "shutdown_rejected" {
		t.Errorf("Type = %q, want shutdown_rejected", msg.Type)
	}
	if msg.Reason != "still processing" {
		t.Errorf("Reason = %q, want still processing", msg.Reason)
	}
}

func TestIsShutdownRejected_Valid(t *testing.T) {
	msg := CreateShutdownRejectedMessage("sd-1", "worker", "busy")
	data, _ := json.Marshal(msg)
	parsed, ok := IsShutdownRejected(string(data))
	if !ok {
		t.Fatal("expected shutdown_rejected")
	}
	if parsed.Reason != "busy" {
		t.Errorf("Reason = %q, want busy", parsed.Reason)
	}
}

// ── ShutdownResponse (legacy unified) ─────────────────────────────────────

func TestIsShutdownResponse_Valid(t *testing.T) {
	// Legacy format from send_message_prompt.go guidance
	msg := ShutdownResponseMessage{
		Type:      "shutdown_response",
		RequestID: "sd-1",
		Approve:   true,
	}
	data, _ := json.Marshal(msg)
	parsed, ok := IsShutdownResponse(string(data))
	if !ok {
		t.Fatal("expected shutdown_response")
	}
	if !parsed.Approve {
		t.Error("Approve should be true")
	}
}

func TestIsShutdownResponse_LegacyFormat(t *testing.T) {
	// Exact format from send_message_prompt.go guidance
	legacy := `{"type":"shutdown_response","request_id":"sd-1","approve":true}`
	parsed, ok := IsShutdownResponse(legacy)
	if !ok {
		t.Fatal("expected shutdown_response from legacy format")
	}
	if !parsed.Approve {
		t.Error("Approve should be true")
	}
	if parsed.RequestID != "sd-1" {
		t.Errorf("RequestID = %q, want sd-1", parsed.RequestID)
	}
}

func TestIsShutdownResponse_Reject(t *testing.T) {
	legacy := `{"type":"shutdown_response","request_id":"sd-2","approve":false}`
	parsed, ok := IsShutdownResponse(legacy)
	if !ok {
		t.Fatal("expected shutdown_response")
	}
	if parsed.Approve {
		t.Error("Approve should be false")
	}
}

// ── PlanApproval ──────────────────────────────────────────────────────────

func TestIsPlanApprovalRequest_Valid(t *testing.T) {
	msg := PlanApprovalRequestMessage{
		Type:         "plan_approval_request",
		From:         "worker",
		Timestamp:    "2026-05-04T10:00:00Z",
		PlanFilePath: "/tmp/plan.md",
		PlanContent:  "Build feature X",
		RequestID:    "plan-1",
	}
	data, _ := json.Marshal(msg)
	parsed, ok := IsPlanApprovalRequest(string(data))
	if !ok {
		t.Fatal("expected plan_approval_request")
	}
	if parsed.PlanFilePath != "/tmp/plan.md" {
		t.Errorf("PlanFilePath = %q, want /tmp/plan.md", parsed.PlanFilePath)
	}
}

func TestIsPlanApprovalRequest_NotPlan(t *testing.T) {
	_, ok := IsPlanApprovalRequest(`{"type": "other"}`)
	if ok {
		t.Error("should not match non-plan_approval_request type")
	}
}

func TestIsPlanApprovalResponse_Valid(t *testing.T) {
	msg := PlanApprovalResponseMessage{
		Type:      "plan_approval_response",
		RequestID: "plan-1",
		Approve:   true,
		Feedback:  "looks good",
		Timestamp: "2026-05-04T10:00:00Z",
	}
	data, _ := json.Marshal(msg)
	parsed, ok := IsPlanApprovalResponse(string(data))
	if !ok {
		t.Fatal("expected plan_approval_response")
	}
	if !parsed.Approve {
		t.Error("Approve should be true")
	}
}

// Verify plan_approval_response accepts snake_case legacy format from prompt guidance.
func TestIsPlanApprovalResponse_LegacyFormat(t *testing.T) {
	// This is the format instructed in send_message_prompt.go
	legacy := `{"type":"plan_approval_response","request_id":"plan-1","approve":false,"feedback":"add error handling"}`
	parsed, ok := IsPlanApprovalResponse(legacy)
	if !ok {
		t.Fatal("expected plan_approval_response from legacy format")
	}
	if parsed.Approve {
		t.Error("Approve should be false from legacy format")
	}
	if parsed.RequestID != "plan-1" {
		t.Errorf("RequestID = %q, want plan-1", parsed.RequestID)
	}
}

// ── TaskAssignment ────────────────────────────────────────────────────────

func TestIsTaskAssignment_Valid(t *testing.T) {
	msg := TaskAssignmentMessage{
		Type:        "task_assignment",
		TaskID:      "task-1",
		Subject:     "Fix bug",
		Description: "Fix the null pointer in handler.go",
		AssignedBy:  "leader",
		Timestamp:   "2026-05-04T10:00:00Z",
	}
	data, _ := json.Marshal(msg)
	parsed, ok := IsTaskAssignment(string(data))
	if !ok {
		t.Fatal("expected task_assignment")
	}
	if parsed.Subject != "Fix bug" {
		t.Errorf("Subject = %q, want Fix bug", parsed.Subject)
	}
}

func TestIsTaskAssignment_NotTask(t *testing.T) {
	_, ok := IsTaskAssignment(`{"type": "shutdown_request"}`)
	if ok {
		t.Error("should not match non-task_assignment type")
	}
}

// ── TeamPermissionUpdate ──────────────────────────────────────────────────

func TestIsTeamPermissionUpdate_Valid(t *testing.T) {
	msg := TeamPermissionUpdateMessage{
		Type: "team_permission_update",
		PermissionUpdate: TeamPermissionUpdatePayload{
			Type: "addRules",
			Rules: []TeamPermissionUpdateRule{
				{ToolName: "BashTool", RuleContent: "allow /tmp/*"},
			},
			Behavior:    "allow",
			Destination: "session",
		},
		DirectoryPath: "/tmp/work",
		ToolName:      "BashTool",
	}
	data, _ := json.Marshal(msg)
	parsed, ok := IsTeamPermissionUpdate(string(data))
	if !ok {
		t.Fatal("expected team_permission_update")
	}
	if parsed.DirectoryPath != "/tmp/work" {
		t.Errorf("DirectoryPath = %q, want /tmp/work", parsed.DirectoryPath)
	}
}

// ── ModeSetRequest ────────────────────────────────────────────────────────

func TestCreateModeSetRequestMessage(t *testing.T) {
	msg := CreateModeSetRequestMessage("acceptEdits", "leader")
	if msg.Type != "mode_set_request" {
		t.Errorf("Type = %q, want mode_set_request", msg.Type)
	}
	if msg.Mode != "acceptEdits" {
		t.Errorf("Mode = %q, want acceptEdits", msg.Mode)
	}
}

func TestIsModeSetRequest_Valid(t *testing.T) {
	msg := CreateModeSetRequestMessage("bypassPermissions", "leader")
	data, _ := json.Marshal(msg)
	parsed, ok := IsModeSetRequest(string(data))
	if !ok {
		t.Fatal("expected mode_set_request")
	}
	if parsed.Mode != "bypassPermissions" {
		t.Errorf("Mode = %q, want bypassPermissions", parsed.Mode)
	}
}

func TestIsModeSetRequest_NotModeSet(t *testing.T) {
	_, ok := IsModeSetRequest(`{"type": "idle_notification"}`)
	if ok {
		t.Error("should not match non-mode_set_request type")
	}
}

// ── IsStructuredProtocolMessage Router ────────────────────────────────────

func TestIsStructuredProtocolMessage_AllTypes(t *testing.T) {
	tests := []struct {
		typeName string
		message  string
	}{
		{"permission_request", `{"type":"permission_request","request_id":"r1","agent_id":"a1","tool_name":"t","tool_use_id":"t1","description":"d","input":{},"permission_suggestions":[]}`},
		{"permission_response", `{"type":"permission_response","request_id":"r1","subtype":"success"}`},
		{"sandbox_permission_request", `{"type":"sandbox_permission_request","requestId":"r1","workerId":"w1","workerName":"wn","hostPattern":{"host":"h"},"createdAt":1}`},
		{"sandbox_permission_response", `{"type":"sandbox_permission_response","requestId":"r1","host":"h","allow":true,"timestamp":"t"}`},
		{"shutdown_request", `{"type":"shutdown_request","requestId":"r1","from":"leader","timestamp":"t"}`},
		{"shutdown_approved", `{"type":"shutdown_approved","requestId":"r1","from":"worker","timestamp":"t"}`},
		{"shutdown_response", `{"type":"shutdown_response","request_id":"r1","approve":true}`},
		{"team_permission_update", `{"type":"team_permission_update","permissionUpdate":{"type":"addRules","rules":[],"behavior":"allow","destination":"session"},"directoryPath":"/x","toolName":"t"}`},
		{"mode_set_request", `{"type":"mode_set_request","mode":"acceptEdits","from":"leader"}`},
		{"plan_approval_request", `{"type":"plan_approval_request","from":"w","timestamp":"t","planFilePath":"p","planContent":"c","requestId":"r1"}`},
		{"plan_approval_response", `{"type":"plan_approval_response","requestId":"r1","approved":true,"timestamp":"t"}`},
	}

	for _, tt := range tests {
		t.Run(tt.typeName, func(t *testing.T) {
			if !IsStructuredProtocolMessage(tt.message) {
				t.Errorf("IsStructuredProtocolMessage should return true for %s", tt.typeName)
			}
		})
	}
}

func TestIsStructuredProtocolMessage_NotStructured(t *testing.T) {
	// Plain text should not be structured
	if IsStructuredProtocolMessage("hello world") {
		t.Error("plain text should not be structured")
	}
	// Unknown type
	if IsStructuredProtocolMessage(`{"type":"unknown_type"}`) {
		t.Error("unknown type should not be structured")
	}
	// idle_notification is NOT in the structured set (it's displayed as context)
	if IsStructuredProtocolMessage(`{"type":"idle_notification","from":"a","timestamp":"t"}`) {
		t.Error("idle_notification should not be in the structured set")
	}
	// task_assignment is NOT in the structured set
	if IsStructuredProtocolMessage(`{"type":"task_assignment","taskId":"t1","subject":"s","description":"d","assignedBy":"a","timestamp":"t"}`) {
		t.Error("task_assignment should not be in the structured set")
	}
	// shutdown_rejected is NOT in the structured set
	if IsStructuredProtocolMessage(`{"type":"shutdown_rejected","requestId":"r1","from":"w","reason":"busy","timestamp":"t"}`) {
		t.Error("shutdown_rejected should not be in the structured set")
	}
}

func TestIsStructuredProtocolMessage_Empty(t *testing.T) {
	if IsStructuredProtocolMessage("") {
		t.Error("empty string should not be structured")
	}
}

func TestIsStructuredProtocolMessage_InvalidJSON(t *testing.T) {
	if IsStructuredProtocolMessage("{invalid") {
		t.Error("invalid JSON should not be structured")
	}
}
