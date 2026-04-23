package remote

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/config"
	"github.com/sheepzhao/claude-code-go/pkg/sdk"
)

type mockSender struct {
	sent [][]byte
}

func (m *mockSender) Send(data []byte) error {
	m.sent = append(m.sent, append([]byte(nil), data...))
	return nil
}

func TestSessionManagerHandleControlRequest(t *testing.T) {
	sender := &mockSender{}
	var receivedReq *sdk.ControlPermissionRequest
	var receivedID string

	sm := NewSessionManager(config.RemoteSessionConfig{}, sender, SessionCallbacks{
		OnPermissionRequest: func(req *sdk.ControlPermissionRequest, requestID string) {
			receivedReq = req
			receivedID = requestID
		},
	})

	payload := `{"type":"control_request","request_id":"req_1","request":{"subtype":"can_use_tool","tool_name":"Bash","input":{"command":"ls"},"tool_use_id":"toolu_1"}}`
	sm.HandleEvent(Event{Data: []byte(payload)})

	if receivedID != "req_1" {
		t.Errorf("requestID = %v, want req_1", receivedID)
	}
	if receivedReq == nil {
		t.Fatal("expected permission request callback")
	}
	if receivedReq.ToolName != "Bash" {
		t.Errorf("toolName = %v, want Bash", receivedReq.ToolName)
	}
	if receivedReq.ToolUseID != "toolu_1" {
		t.Errorf("toolUseID = %v, want toolu_1", receivedReq.ToolUseID)
	}
	if sm.PendingCount() != 1 {
		t.Errorf("pendingCount = %v, want 1", sm.PendingCount())
	}
}

func TestSessionManagerHandleUnknownControlRequest(t *testing.T) {
	sender := &mockSender{}
	sm := NewSessionManager(config.RemoteSessionConfig{}, sender, SessionCallbacks{})

	payload := `{"type":"control_request","request_id":"req_1","request":{"subtype":"set_model","model":"claude-3"}}`
	sm.HandleEvent(Event{Data: []byte(payload)})

	if len(sender.sent) != 1 {
		t.Fatalf("expected 1 response, got %d", len(sender.sent))
	}

	var resp sdk.ControlResponse
	if err := json.Unmarshal(sender.sent[0], &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Response.Subtype != "error" {
		t.Errorf("response.subtype = %v, want error", resp.Response.Subtype)
	}
	if resp.Response.RequestID != "req_1" {
		t.Errorf("response.request_id = %v, want req_1", resp.Response.RequestID)
	}
}

func TestSessionManagerHandleControlCancelRequest(t *testing.T) {
	sender := &mockSender{}
	var cancelledID string
	var cancelledToolUseID string

	sm := NewSessionManager(config.RemoteSessionConfig{}, sender, SessionCallbacks{
		OnPermissionCancelled: func(requestID string, toolUseID string) {
			cancelledID = requestID
			cancelledToolUseID = toolUseID
		},
	})

	// First add a pending request
	payload := `{"type":"control_request","request_id":"req_1","request":{"subtype":"can_use_tool","tool_name":"Bash","input":{"command":"ls"},"tool_use_id":"toolu_1"}}`
	sm.HandleEvent(Event{Data: []byte(payload)})

	// Then cancel it
	cancelPayload := `{"type":"control_cancel_request","request_id":"req_1"}`
	sm.HandleEvent(Event{Data: []byte(cancelPayload)})

	if cancelledID != "req_1" {
		t.Errorf("cancelledID = %v, want req_1", cancelledID)
	}
	if cancelledToolUseID != "toolu_1" {
		t.Errorf("cancelledToolUseID = %v, want toolu_1", cancelledToolUseID)
	}
	if sm.PendingCount() != 0 {
		t.Errorf("pendingCount = %v, want 0", sm.PendingCount())
	}
}

func TestSessionManagerHandleSDKMessage(t *testing.T) {
	sender := &mockSender{}
	var receivedData []byte

	sm := NewSessionManager(config.RemoteSessionConfig{}, sender, SessionCallbacks{
		OnSDKMessage: func(data []byte) {
			receivedData = append([]byte(nil), data...)
		},
	})

	payload := `{"type":"assistant","message":{"role":"assistant","content":"hello"}}`
	sm.HandleEvent(Event{Data: []byte(payload)})

	if string(receivedData) != payload {
		t.Errorf("receivedData = %v, want %v", string(receivedData), payload)
	}
}

func TestSessionManagerRespondToPermissionRequestAllow(t *testing.T) {
	sender := &mockSender{}
	sm := NewSessionManager(config.RemoteSessionConfig{}, sender, SessionCallbacks{})

	// Add pending request
	payload := `{"type":"control_request","request_id":"req_1","request":{"subtype":"can_use_tool","tool_name":"Bash","input":{"command":"ls"},"tool_use_id":"toolu_1"}}`
	sm.HandleEvent(Event{Data: []byte(payload)})

	err := sm.RespondToPermissionRequest("req_1", sdk.PermissionResponse{
		Behavior:     "allow",
		UpdatedInput: map[string]any{"command": "ls -la"},
	})
	if err != nil {
		t.Fatalf("RespondToPermissionRequest error = %v", err)
	}

	if sm.PendingCount() != 0 {
		t.Errorf("pendingCount = %v, want 0", sm.PendingCount())
	}
	if len(sender.sent) != 1 {
		t.Fatalf("expected 1 response sent, got %d", len(sender.sent))
	}

	var resp sdk.ControlResponse
	if err := json.Unmarshal(sender.sent[0], &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Response.Subtype != "success" {
		t.Errorf("response.subtype = %v, want success", resp.Response.Subtype)
	}
	if resp.Response.Response["behavior"] != "allow" {
		t.Errorf("behavior = %v, want allow", resp.Response.Response["behavior"])
	}
}

func TestSessionManagerRespondToPermissionRequestDeny(t *testing.T) {
	sender := &mockSender{}
	sm := NewSessionManager(config.RemoteSessionConfig{}, sender, SessionCallbacks{})

	payload := `{"type":"control_request","request_id":"req_1","request":{"subtype":"can_use_tool","tool_name":"Bash","input":{"command":"ls"},"tool_use_id":"toolu_1"}}`
	sm.HandleEvent(Event{Data: []byte(payload)})

	err := sm.RespondToPermissionRequest("req_1", sdk.PermissionResponse{
		Behavior: "deny",
		Message:  "user denied",
	})
	if err != nil {
		t.Fatalf("RespondToPermissionRequest error = %v", err)
	}

	var resp sdk.ControlResponse
	if err := json.Unmarshal(sender.sent[0], &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Response.Response["behavior"] != "deny" {
		t.Errorf("behavior = %v, want deny", resp.Response.Response["behavior"])
	}
	if resp.Response.Response["message"] != "user denied" {
		t.Errorf("message = %v, want user denied", resp.Response.Response["message"])
	}
}

func TestSessionManagerRespondToMissingPermissionRequest(t *testing.T) {
	sender := &mockSender{}
	sm := NewSessionManager(config.RemoteSessionConfig{}, sender, SessionCallbacks{})

	err := sm.RespondToPermissionRequest("req_missing", sdk.PermissionResponse{
		Behavior: "allow",
	})
	if err == nil {
		t.Fatal("expected error for missing permission request")
	}
}

func TestSessionManagerSendInterrupt(t *testing.T) {
	sender := &mockSender{}
	sm := NewSessionManager(config.RemoteSessionConfig{}, sender, SessionCallbacks{})

	if err := sm.SendInterrupt(); err != nil {
		t.Fatalf("SendInterrupt error = %v", err)
	}

	if len(sender.sent) != 1 {
		t.Fatalf("expected 1 request sent, got %d", len(sender.sent))
	}

	var req sdk.ControlRequest
	if err := json.Unmarshal(sender.sent[0], &req); err != nil {
		t.Fatalf("unmarshal request: %v", err)
	}
	if req.Type != "control_request" {
		t.Errorf("type = %v, want control_request", req.Type)
	}

	var inner sdk.ControlInterruptRequest
	if err := json.Unmarshal(req.Request, &inner); err != nil {
		t.Fatalf("unmarshal inner: %v", err)
	}
	if inner.Subtype != "interrupt" {
		t.Errorf("subtype = %v, want interrupt", inner.Subtype)
	}
}

func TestSessionManagerDisconnect(t *testing.T) {
	sender := &mockSender{}
	sm := NewSessionManager(config.RemoteSessionConfig{}, sender, SessionCallbacks{})

	payload := `{"type":"control_request","request_id":"req_1","request":{"subtype":"can_use_tool","tool_name":"Bash","input":{},"tool_use_id":"toolu_1"}}`
	sm.HandleEvent(Event{Data: []byte(payload)})

	if sm.PendingCount() != 1 {
		t.Errorf("pendingCount = %v, want 1", sm.PendingCount())
	}

	sm.Disconnect()

	if sm.PendingCount() != 0 {
		t.Errorf("pendingCount after disconnect = %v, want 0", sm.PendingCount())
	}
}

func TestSessionManagerNilSafety(t *testing.T) {
	var sm *SessionManager

	sm.HandleEvent(Event{Data: []byte(`{}`)})

	if err := sm.RespondToPermissionRequest("x", sdk.PermissionResponse{}); err == nil {
		t.Error("expected error for nil session manager")
	}

	if err := sm.SendInterrupt(); err == nil {
		t.Error("expected error for nil session manager")
	}

	sm.Disconnect()

	if sm.PendingCount() != 0 {
		t.Error("expected 0 pending for nil session manager")
	}
}

func TestSessionManagerSenderError(t *testing.T) {
	errSender := &mockSender{}
	// Override Send to return an error by wrapping in a custom type
	failingSender := &failingMockSender{err: errors.New("send failed")}

	sm := NewSessionManager(config.RemoteSessionConfig{}, failingSender, SessionCallbacks{})

	// Add pending request first so RespondToPermissionRequest tries to send
	payload := `{"type":"control_request","request_id":"req_1","request":{"subtype":"can_use_tool","tool_name":"Bash","input":{},"tool_use_id":"toolu_1"}}`
	sm.HandleEvent(Event{Data: []byte(payload)})

	err := sm.RespondToPermissionRequest("req_1", sdk.PermissionResponse{Behavior: "allow"})
	if err == nil {
		t.Fatal("expected error when sender fails")
	}

	_ = errSender
}

type failingMockSender struct {
	err error
}

func (f *failingMockSender) Send(data []byte) error {
	return f.err
}
