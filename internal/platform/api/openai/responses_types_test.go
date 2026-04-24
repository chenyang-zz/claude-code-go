package openai

import (
	"encoding/json"
	"testing"
)

func TestResponsesRequestSerialization(t *testing.T) {
	req := responsesRequest{
		Model:           "o3-mini",
		Input:           []responsesInputItem{{Role: "user", Content: "hello"}},
		Tools:           []responsesTool{{Type: "function", Function: toolSpecBody{Name: "get_weather"}}},
		Stream:          true,
		MaxOutputTokens: 4096,
	}
	b, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded responsesRequest
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Model != req.Model {
		t.Errorf("model: got %q, want %q", decoded.Model, req.Model)
	}
	if len(decoded.Input) != 1 || decoded.Input[0].Content != "hello" {
		t.Errorf("input mismatch")
	}
	if !decoded.Stream {
		t.Error("stream should be true")
	}
	if decoded.MaxOutputTokens != 4096 {
		t.Errorf("max_output_tokens: got %d, want 4096", decoded.MaxOutputTokens)
	}
}

func TestResponsesResponseDeserialization(t *testing.T) {
	payload := `{
		"id": "resp_123",
		"model": "o3-mini",
		"output": [
			{"type": "message", "id": "msg_1", "role": "assistant", "content": [{"type": "output_text", "text": "Hi!"}]},
			{"type": "function_call", "id": "fc_1", "call_id": "call_1", "name": "get_weather", "arguments": "{\"city\":\"NYC\"}"}
		],
		"usage": {"input_tokens": 10, "output_tokens": 5, "total_tokens": 15}
	}`
	var resp responsesResponse
	if err := json.Unmarshal([]byte(payload), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.ID != "resp_123" {
		t.Errorf("id: got %q, want resp_123", resp.ID)
	}
	if len(resp.Output) != 2 {
		t.Fatalf("output length: got %d, want 2", len(resp.Output))
	}
	if resp.Output[0].Type != "message" || resp.Output[0].Content[0].Text != "Hi!" {
		t.Error("first output item mismatch")
	}
	if resp.Output[1].Type != "function_call" || resp.Output[1].Name != "get_weather" {
		t.Error("second output item mismatch")
	}
	if resp.Usage == nil || resp.Usage.TotalTokens != 15 {
		t.Error("usage mismatch")
	}
}

func TestResponsesStreamEventDeserialization(t *testing.T) {
	cases := []struct {
		name    string
		payload string
		wantTyp string
	}{
		{"created", `{"type":"response.created","response":{"id":"r1"}}`, responsesEventCreated},
		{"output_text_delta", `{"type":"response.output_text.delta","output_index":0,"content_index":0,"delta":"hello"}`, responsesEventOutputTextDelta},
		{"function_args_delta", `{"type":"response.function_call_arguments.delta","output_index":1,"arguments_delta":"{\"a\":1}"}`, responsesEventFunctionCallArgsDelta},
		{"output_item_done", `{"type":"response.output_item.done","output_index":0,"item":{"type":"message","id":"m1"}}`, responsesEventOutputItemDone},
		{"done", `{"type":"response.done","response":{"id":"r1"}}`, responsesEventDone},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var ev responsesStreamEvent
			if err := json.Unmarshal([]byte(tc.payload), &ev); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if ev.Type != tc.wantTyp {
				t.Errorf("type: got %q, want %q", ev.Type, tc.wantTyp)
			}
		})
	}
}
