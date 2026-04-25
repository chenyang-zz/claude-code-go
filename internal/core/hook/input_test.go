package hook

import (
	"encoding/json"
	"testing"
)

func TestElicitationHookInputSerialization(t *testing.T) {
	input := ElicitationHookInput{
		BaseHookInput: BaseHookInput{
			SessionID: "s1",
			CWD:       "/workspace",
		},
		HookEventName:   string(EventElicitation),
		MCPServerName:   "demo",
		Message:         "Need input",
		Mode:            "form",
		ElicitationID:   "elic-1",
		RequestedSchema: map[string]any{"type": "object"},
	}

	data, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded["hook_event_name"] != "Elicitation" {
		t.Fatalf("hook_event_name = %v, want Elicitation", decoded["hook_event_name"])
	}
	if decoded["mcp_server_name"] != "demo" {
		t.Fatalf("mcp_server_name = %v, want demo", decoded["mcp_server_name"])
	}
}

func TestElicitationResultHookInputSerialization(t *testing.T) {
	input := ElicitationResultHookInput{
		BaseHookInput: BaseHookInput{
			SessionID: "s1",
			CWD:       "/workspace",
		},
		HookEventName: string(EventElicitationResult),
		MCPServerName: "demo",
		ElicitationID: "elic-1",
		Mode:         "url",
		Action:       "accept",
		Content:      map[string]any{"token": "abc"},
	}

	data, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded["hook_event_name"] != "ElicitationResult" {
		t.Fatalf("hook_event_name = %v, want ElicitationResult", decoded["hook_event_name"])
	}
	if decoded["action"] != "accept" {
		t.Fatalf("action = %v, want accept", decoded["action"])
	}
}
