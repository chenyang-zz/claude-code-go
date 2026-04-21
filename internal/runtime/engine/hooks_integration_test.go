package engine

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/conversation"
	"github.com/sheepzhao/claude-code-go/internal/core/hook"
	"github.com/sheepzhao/claude-code-go/internal/core/model"
)

// TestSessionStartHooksDispatchedOnce verifies that SessionStart hooks fire
// exactly once on the first runLoop invocation and not on subsequent calls.
func TestSessionStartHooksDispatchedOnce(t *testing.T) {
	client := &fakeModelClient{
		streams: []model.Stream{
			newModelStream(
				model.Event{Type: model.EventTypeTextDelta, Text: "done"},
				model.Event{Type: model.EventTypeDone, StopReason: model.StopReasonEndTurn},
			),
			newModelStream(
				model.Event{Type: model.EventTypeTextDelta, Text: "again"},
				model.Event{Type: model.EventTypeDone, StopReason: model.StopReasonEndTurn},
			),
		},
	}
	hookRunner := &fakeStopHookRunner{}
	runtime := New(client, "claude-sonnet-4-5", nil)
	runtime.TranscriptPath = "/tmp/transcript.jsonl"
	runtime.Hooks = hook.HooksConfig{
		hook.EventSessionStart: []hook.HookMatcher{{
			Hooks: []json.RawMessage{json.RawMessage(`{"type":"command","command":"echo start"}`)},
		}},
	}
	runtime.HookRunner = hookRunner

	// First run — SessionStart should fire.
	out, err := runtime.Run(context.Background(), conversation.RunRequest{
		SessionID: "sess1",
		Input:     "hello",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	for range out {
	}

	// Second run — SessionStart should NOT fire again.
	out, err = runtime.Run(context.Background(), conversation.RunRequest{
		SessionID: "sess1",
		Input:     "hello again",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	for range out {
	}

	sessionStartCalls := 0
	for _, call := range hookRunner.calls {
		if call.event == hook.EventSessionStart {
			sessionStartCalls++
		}
	}
	if sessionStartCalls != 1 {
		t.Fatalf("SessionStart call count = %d, want 1", sessionStartCalls)
	}

	// Verify the input type and fields.
	var startInput *hook.SessionStartHookInput
	for _, call := range hookRunner.calls {
		if call.event == hook.EventSessionStart {
			if si, ok := call.input.(hook.SessionStartHookInput); ok {
				startInput = &si
			}
		}
	}
	if startInput == nil {
		t.Fatal("SessionStart input type mismatch")
	}
	if startInput.Source != "startup" {
		t.Fatalf("SessionStart source = %q, want 'startup'", startInput.Source)
	}
	if startInput.HookEventName != "SessionStart" {
		t.Fatalf("SessionStart hook_event_name = %q, want 'SessionStart'", startInput.HookEventName)
	}
	if startInput.SessionID != "sess1" {
		t.Fatalf("SessionStart session_id = %q, want 'sess1'", startInput.SessionID)
	}
}

func TestSessionStartHooksDispatchedPerSessionID(t *testing.T) {
	client := &fakeModelClient{
		streams: []model.Stream{
			newModelStream(
				model.Event{Type: model.EventTypeTextDelta, Text: "first"},
				model.Event{Type: model.EventTypeDone, StopReason: model.StopReasonEndTurn},
			),
			newModelStream(
				model.Event{Type: model.EventTypeTextDelta, Text: "second"},
				model.Event{Type: model.EventTypeDone, StopReason: model.StopReasonEndTurn},
			),
		},
	}
	hookRunner := &fakeStopHookRunner{}
	runtime := New(client, "claude-sonnet-4-5", nil)
	runtime.TranscriptPath = "/tmp/transcript.jsonl"
	runtime.Hooks = hook.HooksConfig{
		hook.EventSessionStart: []hook.HookMatcher{{
			Hooks: []json.RawMessage{json.RawMessage(`{"type":"command","command":"echo start"}`)},
		}},
	}
	runtime.HookRunner = hookRunner

	out, err := runtime.Run(context.Background(), conversation.RunRequest{
		SessionID: "sess1",
		Input:     "hello",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	for range out {
	}

	out, err = runtime.Run(context.Background(), conversation.RunRequest{
		SessionID: "sess2",
		Input:     "hello again",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	for range out {
	}

	var sessionIDs []string
	for _, call := range hookRunner.calls {
		if call.event != hook.EventSessionStart {
			continue
		}
		input, ok := call.input.(hook.SessionStartHookInput)
		if !ok {
			t.Fatalf("input type = %T, want hook.SessionStartHookInput", call.input)
		}
		sessionIDs = append(sessionIDs, input.SessionID)
	}

	if len(sessionIDs) != 2 {
		t.Fatalf("SessionStart call count = %d, want 2", len(sessionIDs))
	}
	if sessionIDs[0] != "sess1" || sessionIDs[1] != "sess2" {
		t.Fatalf("SessionStart session IDs = %v, want [sess1 sess2]", sessionIDs)
	}
}

func TestSessionStartHooksUseRequestSource(t *testing.T) {
	client := &fakeModelClient{
		streams: []model.Stream{
			newModelStream(
				model.Event{Type: model.EventTypeTextDelta, Text: "done"},
				model.Event{Type: model.EventTypeDone, StopReason: model.StopReasonEndTurn},
			),
		},
	}
	hookRunner := &fakeStopHookRunner{}
	runtime := New(client, "claude-sonnet-4-5", nil)
	runtime.Hooks = hook.HooksConfig{
		hook.EventSessionStart: []hook.HookMatcher{{
			Hooks: []json.RawMessage{json.RawMessage(`{"type":"command","command":"echo start"}`)},
		}},
	}
	runtime.HookRunner = hookRunner

	out, err := runtime.Run(context.Background(), conversation.RunRequest{
		SessionID:          "sess-resume",
		Input:              "resume work",
		SessionStartSource: "resume",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	for range out {
	}

	if len(hookRunner.calls) != 1 {
		t.Fatalf("hook call count = %d, want 1", len(hookRunner.calls))
	}
	input, ok := hookRunner.calls[0].input.(hook.SessionStartHookInput)
	if !ok {
		t.Fatalf("input type = %T, want hook.SessionStartHookInput", hookRunner.calls[0].input)
	}
	if input.Source != "resume" {
		t.Fatalf("source = %q, want resume", input.Source)
	}
}

// TestSessionEndHooksDispatched verifies that RunSessionEndHooks correctly
// dispatches hooks with the provided reason.
func TestSessionEndHooksDispatched(t *testing.T) {
	hookRunner := &fakeStopHookRunner{}
	runtime := New(nil, "claude-sonnet-4-5", nil)
	runtime.TranscriptPath = "/tmp/transcript.jsonl"
	runtime.sessionID = "sess-end"
	runtime.Hooks = hook.HooksConfig{
		hook.EventSessionEnd: []hook.HookMatcher{{
			Hooks: []json.RawMessage{json.RawMessage(`{"type":"command","command":"echo end"}`)},
		}},
	}
	runtime.HookRunner = hookRunner

	runtime.RunSessionEndHooks(context.Background(), "clear", "/workspace")

	if len(hookRunner.calls) != 1 {
		t.Fatalf("call count = %d, want 1", len(hookRunner.calls))
	}
	input, ok := hookRunner.calls[0].input.(hook.SessionEndHookInput)
	if !ok {
		t.Fatalf("input type = %T, want hook.SessionEndHookInput", hookRunner.calls[0].input)
	}
	if input.Reason != "clear" {
		t.Fatalf("reason = %q, want 'clear'", input.Reason)
	}
	if input.HookEventName != "SessionEnd" {
		t.Fatalf("hook_event_name = %q, want 'SessionEnd'", input.HookEventName)
	}
}

// TestSessionEndHooksSkippedWhenNoConfig verifies that SessionEnd hooks
// are not dispatched when no hooks are configured for the event.
func TestSessionEndHooksSkippedWhenNoConfig(t *testing.T) {
	hookRunner := &fakeStopHookRunner{}
	runtime := New(nil, "claude-sonnet-4-5", nil)
	runtime.sessionID = "sess-skip"
	runtime.HookRunner = hookRunner

	runtime.RunSessionEndHooks(context.Background(), "shutdown", "")

	if len(hookRunner.calls) != 0 {
		t.Fatalf("call count = %d, want 0 (no hooks configured)", len(hookRunner.calls))
	}
}

// TestNotificationHooksDispatched verifies that RunNotificationHooks correctly
// dispatches hooks with message, title, and notification type.
func TestNotificationHooksDispatched(t *testing.T) {
	hookRunner := &fakeStopHookRunner{}
	runtime := New(nil, "claude-sonnet-4-5", nil)
	runtime.TranscriptPath = "/tmp/transcript.jsonl"
	runtime.sessionID = "sess-notify"
	runtime.Hooks = hook.HooksConfig{
		hook.EventNotification: []hook.HookMatcher{{
			Hooks: []json.RawMessage{json.RawMessage(`{"type":"command","command":"echo notify"}`)},
		}},
	}
	runtime.HookRunner = hookRunner

	runtime.RunNotificationHooks(context.Background(), "build complete", "CI", "build_done", "/project")

	if len(hookRunner.calls) != 1 {
		t.Fatalf("call count = %d, want 1", len(hookRunner.calls))
	}
	input, ok := hookRunner.calls[0].input.(hook.NotificationHookInput)
	if !ok {
		t.Fatalf("input type = %T, want hook.NotificationHookInput", hookRunner.calls[0].input)
	}
	if input.Message != "build complete" {
		t.Fatalf("message = %q, want 'build complete'", input.Message)
	}
	if input.Title != "CI" {
		t.Fatalf("title = %q, want 'CI'", input.Title)
	}
	if input.NotificationType != "build_done" {
		t.Fatalf("notification_type = %q, want 'build_done'", input.NotificationType)
	}
	if input.HookEventName != "Notification" {
		t.Fatalf("hook_event_name = %q, want 'Notification'", input.HookEventName)
	}
}

// TestNotificationHooksSkippedWhenNoConfig verifies Notification hooks
// are not dispatched when no hooks are configured.
func TestNotificationHooksSkippedWhenNoConfig(t *testing.T) {
	hookRunner := &fakeStopHookRunner{}
	runtime := New(nil, "claude-sonnet-4-5", nil)
	runtime.sessionID = "sess-skip"
	runtime.HookRunner = hookRunner

	runtime.RunNotificationHooks(context.Background(), "test", "", "test", "")

	if len(hookRunner.calls) != 0 {
		t.Fatalf("call count = %d, want 0 (no hooks configured)", len(hookRunner.calls))
	}
}

// TestCompactHooksInputTypes verifies the JSON serialization of the new
// compact hook input types matches the expected TS schema.
func TestCompactHooksInputTypes(t *testing.T) {
	t.Run("PreCompactHookInput", func(t *testing.T) {
		custom := "focus on API changes"
		input := hook.PreCompactHookInput{
			BaseHookInput: hook.BaseHookInput{
				SessionID:      "s1",
				TranscriptPath: "/tmp/t.jsonl",
				CWD:            "/workspace",
			},
			HookEventName:     "PreCompact",
			Trigger:           "auto",
			CustomInstructions: &custom,
		}
		data, err := json.Marshal(input)
		if err != nil {
			t.Fatalf("Marshal error: %v", err)
		}
		var parsed map[string]any
		if err := json.Unmarshal(data, &parsed); err != nil {
			t.Fatalf("Unmarshal error: %v", err)
		}
		if parsed["hook_event_name"] != "PreCompact" {
			t.Fatalf("hook_event_name = %v, want PreCompact", parsed["hook_event_name"])
		}
		if parsed["trigger"] != "auto" {
			t.Fatalf("trigger = %v, want auto", parsed["trigger"])
		}
		if parsed["custom_instructions"] != "focus on API changes" {
			t.Fatalf("custom_instructions = %v, want focus on API changes", parsed["custom_instructions"])
		}
	})

	t.Run("PostCompactHookInput", func(t *testing.T) {
		input := hook.PostCompactHookInput{
			BaseHookInput: hook.BaseHookInput{
				SessionID:      "s1",
				TranscriptPath: "/tmp/t.jsonl",
				CWD:            "/workspace",
			},
			HookEventName:  "PostCompact",
			Trigger:        "manual",
			CompactSummary: "summary text here",
		}
		data, err := json.Marshal(input)
		if err != nil {
			t.Fatalf("Marshal error: %v", err)
		}
		var parsed map[string]any
		if err := json.Unmarshal(data, &parsed); err != nil {
			t.Fatalf("Unmarshal error: %v", err)
		}
		if parsed["hook_event_name"] != "PostCompact" {
			t.Fatalf("hook_event_name = %v, want PostCompact", parsed["hook_event_name"])
		}
		if parsed["trigger"] != "manual" {
			t.Fatalf("trigger = %v, want manual", parsed["trigger"])
		}
		if parsed["compact_summary"] != "summary text here" {
			t.Fatalf("compact_summary = %v, want summary text here", parsed["compact_summary"])
		}
	})

	t.Run("PreCompactHookInputNullInstructions", func(t *testing.T) {
		input := hook.PreCompactHookInput{
			BaseHookInput: hook.BaseHookInput{
				SessionID: "s1",
			},
			HookEventName:      "PreCompact",
			Trigger:            "auto",
			CustomInstructions: nil,
		}
		data, err := json.Marshal(input)
		if err != nil {
			t.Fatalf("Marshal error: %v", err)
		}
		var parsed map[string]any
		if err := json.Unmarshal(data, &parsed); err != nil {
			t.Fatalf("Unmarshal error: %v", err)
		}
		if parsed["custom_instructions"] != nil {
			t.Fatalf("custom_instructions = %v, want nil", parsed["custom_instructions"])
		}
	})
}

func TestPreCompactHooksOnlyFireWhenAutoCompactTriggers(t *testing.T) {
	client := &fakeModelClient{
		streams: []model.Stream{
			newModelStream(
				model.Event{Type: model.EventTypeTextDelta, Text: "done"},
				model.Event{Type: model.EventTypeDone, StopReason: model.StopReasonEndTurn},
			),
		},
	}
	hookRunner := &fakeStopHookRunner{}
	runtime := New(client, "claude-sonnet-4-20250514", nil)
	runtime.AutoCompact = true
	runtime.TranscriptPath = "/tmp/transcript.jsonl"
	runtime.Hooks = hook.HooksConfig{
		hook.EventPreCompact: []hook.HookMatcher{{
			Hooks: []json.RawMessage{json.RawMessage(`{"type":"command","command":"echo pre"}`)},
		}},
	}
	runtime.HookRunner = hookRunner

	out, err := runtime.Run(context.Background(), conversation.RunRequest{
		SessionID: "sess1",
		Input:     "short message",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	for range out {
	}

	for _, call := range hookRunner.calls {
		if call.event == hook.EventPreCompact {
			t.Fatalf("unexpected PreCompact hook call: %+v", call)
		}
	}
}

func TestPreCompactHooksSkipFailedAutoCompact(t *testing.T) {
	streamCalls := 0
	client := &fakeModelClient{
		streamFn: func(ctx context.Context, req model.Request) (model.Stream, error) {
			_ = ctx
			_ = req
			streamCalls++
			if streamCalls == 1 {
				return nil, errors.New("prompt is too long: 250000 tokens > 200000")
			}
			return nil, errors.New("compact failed")
		},
	}
	hookRunner := &fakeStopHookRunner{}
	runtime := New(client, "claude-sonnet-4-20250514", nil)
	runtime.AutoCompact = true
	runtime.TranscriptPath = "/tmp/transcript.jsonl"
	runtime.Hooks = hook.HooksConfig{
		hook.EventPreCompact: []hook.HookMatcher{{
			Hooks: []json.RawMessage{json.RawMessage(`{"type":"command","command":"echo pre"}`)},
		}},
	}
	runtime.HookRunner = hookRunner

	out, err := runtime.Run(context.Background(), conversation.RunRequest{
		SessionID: "sess1",
		Input:     "trigger prompt-too-long recovery",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	for range out {
	}

	if streamCalls != 2 {
		t.Fatalf("Stream() call count = %d, want 2", streamCalls)
	}

	for _, call := range hookRunner.calls {
		if call.event == hook.EventPreCompact {
			t.Fatalf("unexpected PreCompact hook call on failed compaction recovery: %+v", call)
		}
	}
}

// TestNotificationHookInputSerialization verifies the JSON serialization
// of NotificationHookInput.
func TestNotificationHookInputSerialization(t *testing.T) {
	input := hook.NotificationHookInput{
		BaseHookInput: hook.BaseHookInput{
			SessionID: "s1",
		},
		HookEventName:    "Notification",
		Message:          "task done",
		Title:            "Agent",
		NotificationType: "task_complete",
	}
	data, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if parsed["hook_event_name"] != "Notification" {
		t.Fatalf("hook_event_name = %v, want Notification", parsed["hook_event_name"])
	}
	if parsed["message"] != "task done" {
		t.Fatalf("message = %v, want task done", parsed["message"])
	}
	if parsed["notification_type"] != "task_complete" {
		t.Fatalf("notification_type = %v, want task_complete", parsed["notification_type"])
	}
}

// TestSessionStartHookInputSerialization verifies the JSON serialization
// of SessionStartHookInput.
func TestSessionStartHookInputSerialization(t *testing.T) {
	input := hook.SessionStartHookInput{
		BaseHookInput: hook.BaseHookInput{
			SessionID: "s1",
			AgentType: "general-purpose",
		},
		HookEventName: "SessionStart",
		Source:        "resume",
		Model:         "claude-sonnet-4-5",
	}
	data, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if parsed["hook_event_name"] != "SessionStart" {
		t.Fatalf("hook_event_name = %v, want SessionStart", parsed["hook_event_name"])
	}
	if parsed["source"] != "resume" {
		t.Fatalf("source = %v, want resume", parsed["source"])
	}
	if parsed["model"] != "claude-sonnet-4-5" {
		t.Fatalf("model = %v, want claude-sonnet-4-5", parsed["model"])
	}
	if parsed["agent_type"] != "general-purpose" {
		t.Fatalf("agent_type = %v, want general-purpose", parsed["agent_type"])
	}
}

// TestSessionEndHookInputSerialization verifies the JSON serialization
// of SessionEndHookInput.
func TestSessionEndHookInputSerialization(t *testing.T) {
	input := hook.SessionEndHookInput{
		BaseHookInput: hook.BaseHookInput{
			SessionID: "s1",
		},
		HookEventName: "SessionEnd",
		Reason:        "shutdown",
	}
	data, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if parsed["hook_event_name"] != "SessionEnd" {
		t.Fatalf("hook_event_name = %v, want SessionEnd", parsed["hook_event_name"])
	}
	if parsed["reason"] != "shutdown" {
		t.Fatalf("reason = %v, want shutdown", parsed["reason"])
	}
}
