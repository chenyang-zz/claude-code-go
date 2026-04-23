package repl

import (
	"context"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/event"
	"github.com/sheepzhao/claude-code-go/internal/core/tool"
	"github.com/sheepzhao/claude-code-go/pkg/sdk"
)

func TestPermissionBridgeCreateApprovalEvent(t *testing.T) {
	bridge := NewPermissionBridge()

	req := &sdk.ControlPermissionRequest{
		Subtype:     "can_use_tool",
		ToolName:    "Bash",
		Input:       map[string]any{"command": "ls"},
		ToolUseID:   "toolu_1",
		Title:       "Run Bash command",
		Description: "Execute a bash command",
	}

	evt := bridge.CreateApprovalEvent(req, "req_1")

	if evt.Type != event.TypeApprovalRequired {
		t.Errorf("type = %v, want %v", evt.Type, event.TypeApprovalRequired)
	}
	if evt.Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}

	payload, ok := evt.Payload.(event.ApprovalPayload)
	if !ok {
		t.Fatal("payload type mismatch")
	}
	if payload.CallID != "toolu_1" {
		t.Errorf("callID = %v, want toolu_1", payload.CallID)
	}
	if payload.ToolName != "Bash" {
		t.Errorf("toolName = %v, want Bash", payload.ToolName)
	}
	if payload.Action != "use" {
		t.Errorf("action = %v, want use", payload.Action)
	}
	if payload.Message == "" {
		t.Error("expected non-empty message")
	}
}

func TestPermissionBridgeCreateApprovalEventWithBlockedPath(t *testing.T) {
	bridge := NewPermissionBridge()

	req := &sdk.ControlPermissionRequest{
		Subtype:     "can_use_tool",
		ToolName:    "FileWrite",
		Input:       map[string]any{"path": "/etc/passwd"},
		ToolUseID:   "toolu_2",
		BlockedPath: "/etc/passwd",
	}

	evt := bridge.CreateApprovalEvent(req, "req_2")
	payload := evt.Payload.(event.ApprovalPayload)
	if payload.Path != "/etc/passwd" {
		t.Errorf("path = %v, want /etc/passwd", payload.Path)
	}
}

func TestPermissionBridgeCreateApprovalEventFallbackMessage(t *testing.T) {
	bridge := NewPermissionBridge()

	req := &sdk.ControlPermissionRequest{
		Subtype:   "can_use_tool",
		ToolName:  "CustomTool",
		Input:     map[string]any{},
		ToolUseID: "toolu_3",
	}

	evt := bridge.CreateApprovalEvent(req, "req_3")
	payload := evt.Payload.(event.ApprovalPayload)
	if payload.Message == "" {
		t.Error("expected non-empty fallback message")
	}
}

func TestPermissionBridgeCreateToolStub(t *testing.T) {
	bridge := NewPermissionBridge()

	stub := bridge.CreateToolStub("RemoteMCP")
	if stub.Name() != "RemoteMCP" {
		t.Errorf("name = %v, want RemoteMCP", stub.Name())
	}
	if stub.IsReadOnly() {
		t.Error("expected stub to not be read-only")
	}
	if stub.IsConcurrencySafe() {
		t.Error("expected stub to not be concurrency-safe")
	}
	if stub.Description() == "" {
		t.Error("expected non-empty description")
	}

	_, err := stub.Invoke(context.Background(), tool.Call{})
	if err == nil {
		t.Error("expected error from stub Invoke")
	}
}

func TestPermissionBridgeCreateToolStubInputSchema(t *testing.T) {
	bridge := NewPermissionBridge()
	stub := bridge.CreateToolStub("UnknownTool")

	schema := stub.InputSchema()
	if len(schema.Properties) != 0 {
		t.Errorf("expected empty schema, got %d properties", len(schema.Properties))
	}
}
