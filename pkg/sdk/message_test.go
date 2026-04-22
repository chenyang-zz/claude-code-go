package sdk

import (
	"encoding/json"
	"testing"
)

func TestStreamEventJSON(t *testing.T) {
	parentID := "parent_1"
	msg := StreamEvent{
		Base:            Base{Type: "stream_event", UUID: "u1", SessionID: "s1"},
		Event:           map[string]any{"type": "text_delta", "text": "hello"},
		ParentToolUseID: &parentID,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal StreamEvent: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal StreamEvent: %v", err)
	}

	if parsed["type"] != "stream_event" {
		t.Errorf("type = %v, want stream_event", parsed["type"])
	}
	if parsed["uuid"] != "u1" {
		t.Errorf("uuid = %v, want u1", parsed["uuid"])
	}
	if parsed["session_id"] != "s1" {
		t.Errorf("session_id = %v, want s1", parsed["session_id"])
	}
	if parsed["parent_tool_use_id"] != "parent_1" {
		t.Errorf("parent_tool_use_id = %v, want parent_1", parsed["parent_tool_use_id"])
	}
	event, _ := parsed["event"].(map[string]any)
	if event["text"] != "hello" {
		t.Errorf("event.text = %v, want hello", event["text"])
	}
}

func TestAssistantJSON(t *testing.T) {
	errMsg := "rate_limit"
	msg := Assistant{
		Base:            Base{Type: "assistant"},
		Message:         map[string]any{"role": "assistant", "content": "hi"},
		ParentToolUseID: nil,
		Error:           &errMsg,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal Assistant: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal Assistant: %v", err)
	}

	if parsed["type"] != "assistant" {
		t.Errorf("type = %v, want assistant", parsed["type"])
	}
	if parsed["error"] != "rate_limit" {
		t.Errorf("error = %v, want rate_limit", parsed["error"])
	}
}

func TestUserJSON(t *testing.T) {
	msg := User{
		Base:        Base{Type: "user"},
		Message:     map[string]any{"role": "user", "content": "query"},
		IsSynthetic: true,
		Priority:    "now",
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal User: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal User: %v", err)
	}

	if parsed["type"] != "user" {
		t.Errorf("type = %v, want user", parsed["type"])
	}
	if parsed["is_synthetic"] != true {
		t.Errorf("is_synthetic = %v, want true", parsed["is_synthetic"])
	}
	if parsed["priority"] != "now" {
		t.Errorf("priority = %v, want now", parsed["priority"])
	}
}

func TestResultSuccessJSON(t *testing.T) {
	stopReason := "end_turn"
	msg := Result{
		Base:              Base{Type: "result"},
		Subtype:           "success",
		DurationMs:        1234,
		DurationApiMs:     1000,
		IsError:           false,
		NumTurns:          3,
		Result:            "done",
		StopReason:        &stopReason,
		TotalCostUSD:      0.001,
		Usage:             map[string]any{"input_tokens": 100},
		ModelUsage:        map[string]any{"claude-3-sonnet": map[string]any{"requests": 1}},
		PermissionDenials: []any{},
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal Result: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal Result: %v", err)
	}

	if parsed["type"] != "result" {
		t.Errorf("type = %v, want result", parsed["type"])
	}
	if parsed["subtype"] != "success" {
		t.Errorf("subtype = %v, want success", parsed["subtype"])
	}
	if parsed["is_error"] != false {
		t.Errorf("is_error = %v, want false", parsed["is_error"])
	}
	if parsed["num_turns"] != float64(3) {
		t.Errorf("num_turns = %v, want 3", parsed["num_turns"])
	}
}

func TestResultErrorJSON(t *testing.T) {
	msg := Result{
		Base:     Base{Type: "result"},
		Subtype:  "error_during_execution",
		IsError:  true,
		Errors:   []string{"something went wrong"},
		DurationMs: 500,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal Result: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal Result: %v", err)
	}

	if parsed["subtype"] != "error_during_execution" {
		t.Errorf("subtype = %v, want error_during_execution", parsed["subtype"])
	}
	errs, _ := parsed["errors"].([]any)
	if len(errs) != 1 || errs[0] != "something went wrong" {
		t.Errorf("errors = %v, want [something went wrong]", parsed["errors"])
	}
}

func TestSystemInitJSON(t *testing.T) {
	msg := SystemInit{
		Base:              Base{Type: "system"},
		Subtype:           "init",
		ClaudeCodeVersion: "1.0.0",
		CWD:               "/home/user",
		Model:             "claude-3-sonnet",
		PermissionMode:    "default",
		Tools:             []string{"Bash", "Glob"},
		MCPServers:        []MCPServer{{Name: "mcp1", Status: "connected"}},
		SlashCommands:     []string{"/commit"},
		Skills:            []string{"git"},
		Plugins:           []Plugin{{Name: "p1", Path: "/plugin"}},
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal SystemInit: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal SystemInit: %v", err)
	}

	if parsed["type"] != "system" {
		t.Errorf("type = %v, want system", parsed["type"])
	}
	if parsed["subtype"] != "init" {
		t.Errorf("subtype = %v, want init", parsed["subtype"])
	}
	if parsed["claude_code_version"] != "1.0.0" {
		t.Errorf("claude_code_version = %v, want 1.0.0", parsed["claude_code_version"])
	}
	if parsed["permission_mode"] != "default" {
		t.Errorf("permission_mode = %v, want default", parsed["permission_mode"])
	}
	mcpServers, _ := parsed["mcp_servers"].([]any)
	if len(mcpServers) != 1 {
		t.Errorf("mcp_servers length = %v, want 1", len(mcpServers))
	}
}

func TestToolProgressJSON(t *testing.T) {
	parentID := "p1"
	msg := ToolProgress{
		Base:            Base{Type: "tool_progress"},
		ToolUseID:       "toolu_1",
		ToolName:        "Bash",
		ParentToolUseID: &parentID,
		ElapsedTimeSec:  5.5,
		TaskID:          "task_1",
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal ToolProgress: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal ToolProgress: %v", err)
	}

	if parsed["type"] != "tool_progress" {
		t.Errorf("type = %v, want tool_progress", parsed["type"])
	}
	if parsed["tool_use_id"] != "toolu_1" {
		t.Errorf("tool_use_id = %v, want toolu_1", parsed["tool_use_id"])
	}
	if parsed["tool_name"] != "Bash" {
		t.Errorf("tool_name = %v, want Bash", parsed["tool_name"])
	}
	if parsed["elapsed_time_seconds"] != 5.5 {
		t.Errorf("elapsed_time_seconds = %v, want 5.5", parsed["elapsed_time_seconds"])
	}
}

func TestMessageInterface(t *testing.T) {
	msgs := []Message{
		StreamEvent{Base: Base{Type: "stream_event"}},
		Assistant{Base: Base{Type: "assistant"}},
		User{Base: Base{Type: "user"}},
		Result{Base: Base{Type: "result"}},
		SystemInit{Base: Base{Type: "system"}},
		ToolProgress{Base: Base{Type: "tool_progress"}},
	}

	for _, m := range msgs {
		if _, err := json.Marshal(m); err != nil {
			t.Fatalf("marshal %T: %v", m, err)
		}
	}
}
