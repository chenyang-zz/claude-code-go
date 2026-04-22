package event

import (
	"testing"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/model"
	"github.com/sheepzhao/claude-code-go/pkg/sdk"
)

func TestToSDKMessage_MessageDelta(t *testing.T) {
	evt := Event{
		Type:      TypeMessageDelta,
		Timestamp: time.Now(),
		Payload:   MessageDeltaPayload{Text: "hello world"},
	}

	msg := evt.ToSDKMessage()
	if msg == nil {
		t.Fatal("expected non-nil message")
	}

	se, ok := msg.(sdk.StreamEvent)
	if !ok {
		t.Fatalf("expected sdk.StreamEvent, got %T", msg)
	}
	if se.Type != "stream_event" {
		t.Errorf("type = %v, want stream_event", se.Type)
	}
	eventMap, ok := se.Event.(map[string]any)
	if !ok {
		t.Fatalf("expected Event to be map[string]any, got %T", se.Event)
	}
	if eventMap["text"] != "hello world" {
		t.Errorf("event.text = %v, want hello world", eventMap["text"])
	}
}

func TestToSDKMessage_Progress(t *testing.T) {
	evt := Event{
		Type:      TypeProgress,
		Timestamp: time.Now(),
		Payload:   ProgressPayload{ToolUseID: "toolu_1", ParentToolUseID: "parent_1"},
	}

	msg := evt.ToSDKMessage()
	if msg == nil {
		t.Fatal("expected non-nil message")
	}

	tp, ok := msg.(sdk.ToolProgress)
	if !ok {
		t.Fatalf("expected sdk.ToolProgress, got %T", msg)
	}
	if tp.Type != "tool_progress" {
		t.Errorf("type = %v, want tool_progress", tp.Type)
	}
	if tp.ToolUseID != "toolu_1" {
		t.Errorf("tool_use_id = %v, want toolu_1", tp.ToolUseID)
	}
	if tp.ParentToolUseID == nil || *tp.ParentToolUseID != "parent_1" {
		t.Errorf("parent_tool_use_id = %v, want parent_1", tp.ParentToolUseID)
	}
}

func TestToSDKMessage_ProgressNoParent(t *testing.T) {
	evt := Event{
		Type:      TypeProgress,
		Timestamp: time.Now(),
		Payload:   ProgressPayload{ToolUseID: "toolu_1"},
	}

	msg := evt.ToSDKMessage()
	if msg == nil {
		t.Fatal("expected non-nil message")
	}

	tp, ok := msg.(sdk.ToolProgress)
	if !ok {
		t.Fatalf("expected sdk.ToolProgress, got %T", msg)
	}
	if tp.ParentToolUseID != nil {
		t.Errorf("expected nil parent_tool_use_id, got %v", *tp.ParentToolUseID)
	}
}

func TestToSDKMessage_ConversationDone(t *testing.T) {
	evt := Event{
		Type:      TypeConversationDone,
		Timestamp: time.Now(),
		Payload: ConversationDonePayload{
			Usage:      model.Usage{InputTokens: 100, OutputTokens: 50},
			StopReason: "max_tokens",
		},
	}

	msg := evt.ToSDKMessage()
	if msg == nil {
		t.Fatal("expected non-nil message")
	}

	r, ok := msg.(sdk.Result)
	if !ok {
		t.Fatalf("expected sdk.Result, got %T", msg)
	}
	if r.Type != "result" {
		t.Errorf("type = %v, want result", r.Type)
	}
	if r.Subtype != "success" {
		t.Errorf("subtype = %v, want success", r.Subtype)
	}
	if r.IsError != false {
		t.Errorf("is_error = %v, want false", r.IsError)
	}
	if r.StopReason == nil || *r.StopReason != "max_tokens" {
		t.Errorf("stop_reason = %v, want max_tokens", r.StopReason)
	}
}

func TestToSDKMessage_ConversationDoneDefaultsStopReason(t *testing.T) {
	evt := Event{
		Type:      TypeConversationDone,
		Timestamp: time.Now(),
		Payload:   ConversationDonePayload{Usage: model.Usage{InputTokens: 100, OutputTokens: 50}},
	}

	msg := evt.ToSDKMessage()
	if msg == nil {
		t.Fatal("expected non-nil message")
	}

	r, ok := msg.(sdk.Result)
	if !ok {
		t.Fatalf("expected sdk.Result, got %T", msg)
	}
	if r.StopReason == nil || *r.StopReason != "end_turn" {
		t.Errorf("stop_reason = %v, want end_turn", r.StopReason)
	}
}

func TestToSDKMessage_Error(t *testing.T) {
	evt := Event{
		Type:      TypeError,
		Timestamp: time.Now(),
		Payload:   ErrorPayload{Message: "connection failed"},
	}

	msg := evt.ToSDKMessage()
	if msg == nil {
		t.Fatal("expected non-nil message")
	}

	r, ok := msg.(sdk.Result)
	if !ok {
		t.Fatalf("expected sdk.Result, got %T", msg)
	}
	if r.Subtype != "error_during_execution" {
		t.Errorf("subtype = %v, want error_during_execution", r.Subtype)
	}
	if r.IsError != true {
		t.Errorf("is_error = %v, want true", r.IsError)
	}
	if len(r.Errors) != 1 || r.Errors[0] != "connection failed" {
		t.Errorf("errors = %v, want [connection failed]", r.Errors)
	}
}

func TestToSDKMessage_NoMapping(t *testing.T) {
	noMappingTypes := []Type{
		TypeToolCallStarted,
		TypeToolCallFinished,
		TypeApprovalRequired,
		TypeUsage,
		TypeModelFallback,
		TypeRetryAttempted,
		TypeCompactDone,
	}

	for _, typ := range noMappingTypes {
		evt := Event{Type: typ, Timestamp: time.Now(), Payload: nil}
		msg := evt.ToSDKMessage()
		if msg != nil {
			t.Errorf("type %s: expected nil message, got %T", typ, msg)
		}
	}
}

func TestToSDKMessage_WrongPayload(t *testing.T) {
	// Event with mismatched payload should return nil
	evt := Event{
		Type:      TypeMessageDelta,
		Timestamp: time.Now(),
		Payload:   "wrong type",
	}
	msg := evt.ToSDKMessage()
	if msg != nil {
		t.Errorf("expected nil for wrong payload, got %T", msg)
	}
}
