package sdk

import (
	"encoding/json"
	"testing"
)

func TestControlRequestJSON(t *testing.T) {
	inner := map[string]any{
		"subtype":     "can_use_tool",
		"tool_name":   "Bash",
		"input":       map[string]any{"command": "ls"},
		"tool_use_id": "toolu_1",
	}
	innerData, _ := json.Marshal(inner)

	req := ControlRequest{
		Type:      "control_request",
		RequestID: "req_1",
		Request:   innerData,
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal ControlRequest: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal ControlRequest: %v", err)
	}

	if parsed["type"] != "control_request" {
		t.Errorf("type = %v, want control_request", parsed["type"])
	}
	if parsed["request_id"] != "req_1" {
		t.Errorf("request_id = %v, want req_1", parsed["request_id"])
	}
	request, _ := parsed["request"].(map[string]any)
	if request["subtype"] != "can_use_tool" {
		t.Errorf("request.subtype = %v, want can_use_tool", request["subtype"])
	}
}

func TestControlPermissionRequestJSON(t *testing.T) {
	req := ControlPermissionRequest{
		Subtype:   "can_use_tool",
		ToolName:  "Bash",
		Input:     map[string]any{"command": "ls"},
		ToolUseID: "toolu_1",
		Title:     "Run Bash",
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal ControlPermissionRequest: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal ControlPermissionRequest: %v", err)
	}

	if parsed["subtype"] != "can_use_tool" {
		t.Errorf("subtype = %v, want can_use_tool", parsed["subtype"])
	}
	if parsed["tool_name"] != "Bash" {
		t.Errorf("tool_name = %v, want Bash", parsed["tool_name"])
	}
	if parsed["tool_use_id"] != "toolu_1" {
		t.Errorf("tool_use_id = %v, want toolu_1", parsed["tool_use_id"])
	}
	if parsed["title"] != "Run Bash" {
		t.Errorf("title = %v, want Run Bash", parsed["title"])
	}
}

func TestControlResponseSuccessJSON(t *testing.T) {
	resp := ControlResponse{
		Type: "control_response",
		Response: ControlResponseInner{
			Subtype:   "success",
			RequestID: "req_1",
			Response: map[string]any{
				"behavior":     "allow",
				"updatedInput": map[string]any{"command": "ls -la"},
			},
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal ControlResponse: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal ControlResponse: %v", err)
	}

	if parsed["type"] != "control_response" {
		t.Errorf("type = %v, want control_response", parsed["type"])
	}
	response, _ := parsed["response"].(map[string]any)
	if response["subtype"] != "success" {
		t.Errorf("response.subtype = %v, want success", response["subtype"])
	}
	if response["request_id"] != "req_1" {
		t.Errorf("response.request_id = %v, want req_1", response["request_id"])
	}
}

func TestControlResponseErrorJSON(t *testing.T) {
	resp := ControlResponse{
		Type: "control_response",
		Response: ControlResponseInner{
			Subtype:   "error",
			RequestID: "req_1",
			Error:     "unsupported subtype",
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal ControlResponse: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal ControlResponse: %v", err)
	}

	response, _ := parsed["response"].(map[string]any)
	if response["subtype"] != "error" {
		t.Errorf("response.subtype = %v, want error", response["subtype"])
	}
	if response["error"] != "unsupported subtype" {
		t.Errorf("response.error = %v, want unsupported subtype", response["error"])
	}
}

func TestControlCancelRequestJSON(t *testing.T) {
	req := ControlCancelRequest{
		Type:      "control_cancel_request",
		RequestID: "req_1",
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal ControlCancelRequest: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal ControlCancelRequest: %v", err)
	}

	if parsed["type"] != "control_cancel_request" {
		t.Errorf("type = %v, want control_cancel_request", parsed["type"])
	}
	if parsed["request_id"] != "req_1" {
		t.Errorf("request_id = %v, want req_1", parsed["request_id"])
	}
}

func TestPermissionResponse(t *testing.T) {
	allow := PermissionResponse{
		Behavior:     "allow",
		UpdatedInput: map[string]any{"command": "ls -la"},
	}
	if allow.Behavior != "allow" {
		t.Errorf("behavior = %v, want allow", allow.Behavior)
	}
	if allow.UpdatedInput["command"] != "ls -la" {
		t.Errorf("updatedInput.command = %v, want ls -la", allow.UpdatedInput["command"])
	}

	deny := PermissionResponse{
		Behavior: "deny",
		Message:  "user denied",
	}
	if deny.Behavior != "deny" {
		t.Errorf("behavior = %v, want deny", deny.Behavior)
	}
	if deny.Message != "user denied" {
		t.Errorf("message = %v, want user denied", deny.Message)
	}
}
