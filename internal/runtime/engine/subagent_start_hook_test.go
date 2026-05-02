package engine

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/hook"
)

// TestSubagentStartHookInputType verifies the JSON serialization of
// SubagentStartHookInput matches the schema expected by hook commands
// (hook_event_name="SubagentStart", agent_id, agent_type, BaseHookInput fields).
func TestSubagentStartHookInputType(t *testing.T) {
	input := hook.SubagentStartHookInput{
		BaseHookInput: hook.BaseHookInput{
			SessionID:      "sess-subagent",
			TranscriptPath: "/tmp/transcript.jsonl",
			CWD:            "/repo",
		},
		HookEventName: "SubagentStart",
		AgentID:       "agent-001",
		AgentType:     "general-purpose",
	}

	data, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if parsed["hook_event_name"] != "SubagentStart" {
		t.Fatalf("hook_event_name = %v, want SubagentStart", parsed["hook_event_name"])
	}
	if parsed["agent_id"] != "agent-001" {
		t.Fatalf("agent_id = %v, want agent-001", parsed["agent_id"])
	}
	if parsed["agent_type"] != "general-purpose" {
		t.Fatalf("agent_type = %v, want general-purpose", parsed["agent_type"])
	}
	if parsed["session_id"] != "sess-subagent" {
		t.Fatalf("session_id = %v, want sess-subagent", parsed["session_id"])
	}
	if parsed["transcript_path"] != "/tmp/transcript.jsonl" {
		t.Fatalf("transcript_path = %v", parsed["transcript_path"])
	}
	if parsed["cwd"] != "/repo" {
		t.Fatalf("cwd = %v, want /repo", parsed["cwd"])
	}
}

// TestSubagentStartHooksDispatched verifies that RunSubagentStartHooks dispatches
// hooks with the correct event name, agent_id, and agent_type when a sub-agent
// starts. It validates that the hook runner receives a properly populated
// SubagentStartHookInput.
func TestSubagentStartHooksDispatched(t *testing.T) {
	hookRunner := &fakeStopHookRunner{
		results: []hook.HookResult{{ExitCode: 0, Stdout: "ok"}},
	}
	runtime := New(nil, "claude-sonnet-4-5", nil)
	runtime.TranscriptPath = "/tmp/transcript.jsonl"
	runtime.sessionID = "sess-subagent"
	runtime.Hooks = hook.HooksConfig{
		hook.EventSubagentStart: []hook.HookMatcher{{
			Matcher: "general-purpose",
			Hooks:   []json.RawMessage{json.RawMessage(`{"type":"command","command":"echo hi"}`)},
		}},
	}
	runtime.HookRunner = hookRunner

	results, blocked, blockingMessages, additionalContext := runtime.RunSubagentStartHooks(
		context.Background(), "agent-001", "general-purpose", "/workspace",
	)

	if len(hookRunner.calls) != 1 {
		t.Fatalf("call count = %d, want 1", len(hookRunner.calls))
	}
	if blocked {
		t.Fatalf("blocked = true, want false for exit-0 hook")
	}
	if len(blockingMessages) != 0 {
		t.Fatalf("blockingMessages = %v, want empty when not blocked", blockingMessages)
	}
	if additionalContext != "" {
		t.Fatalf("additionalContext = %q, want empty", additionalContext)
	}
	if len(results) != 1 || results[0].ExitCode != 0 {
		t.Fatalf("results = %+v, want one exit-0 result", results)
	}

	input, ok := hookRunner.calls[0].input.(hook.SubagentStartHookInput)
	if !ok {
		t.Fatalf("input type = %T, want hook.SubagentStartHookInput", hookRunner.calls[0].input)
	}
	if input.AgentID != "agent-001" {
		t.Fatalf("agent_id = %q, want 'agent-001'", input.AgentID)
	}
	if input.AgentType != "general-purpose" {
		t.Fatalf("agent_type = %q, want 'general-purpose'", input.AgentType)
	}
	if input.HookEventName != "SubagentStart" {
		t.Fatalf("hook_event_name = %q, want SubagentStart", input.HookEventName)
	}
	if input.CWD != "/workspace" {
		t.Fatalf("cwd = %q, want /workspace", input.CWD)
	}
}

// TestSubagentStartHooksAdditionalContext verifies that RunSubagentStartHooks
// collects additionalContext from successful hook stdout and returns it as a
// joined string. This matches the TS runAgent.ts pattern where hook context is
// injected as a user message into the sub-agent's initial prompt.
func TestSubagentStartHooksAdditionalContext(t *testing.T) {
	hookRunner := &fakeStopHookRunner{
		results: []hook.HookResult{
			{
				ExitCode: 0,
				ParsedOutput: &hook.HookOutput{
					HookSpecificOutput: json.RawMessage(
						`{"hookEventName":"SubagentStart","additionalContext":"Context from hook A"}`,
					),
				},
			},
		},
	}
	runtime := New(nil, "claude-sonnet-4-5", nil)
	runtime.sessionID = "sess-ctx"
	runtime.Hooks = hook.HooksConfig{
		hook.EventSubagentStart: []hook.HookMatcher{{
			Matcher: "general-purpose",
			Hooks:   []json.RawMessage{json.RawMessage(`{"type":"command","command":"ctx-inject"}`)},
		}},
	}
	runtime.HookRunner = hookRunner

	results, blocked, blockingMessages, additionalContext := runtime.RunSubagentStartHooks(
		context.Background(), "agent-ctx", "general-purpose", "/repo",
	)

	if blocked {
		t.Fatalf("blocked = true, want false")
	}
	if len(blockingMessages) != 0 {
		t.Fatalf("blockingMessages = %v, want empty", blockingMessages)
	}
	if additionalContext != "Context from hook A" {
		t.Fatalf("additionalContext = %q, want 'Context from hook A'", additionalContext)
	}
	if len(results) != 1 {
		t.Fatalf("results length = %d, want 1", len(results))
	}
}

// TestSubagentStartHooksMultipleResults verifies that when multiple hooks are
// configured for SubagentStart, additional contexts are collected and joined,
// and blocking hooks correctly surface their stderr.
func TestSubagentStartHooksMultipleResults(t *testing.T) {
	hookRunner := &fakeStopHookRunner{
		results: []hook.HookResult{
			{
				ExitCode: 0,
				ParsedOutput: &hook.HookOutput{
					HookSpecificOutput: json.RawMessage(
						`{"hookEventName":"SubagentStart","additionalContext":"Context 1"}`,
					),
				},
			},
			{
				ExitCode: 2,
				Stderr:   "blocked by policy hook",
			},
			{
				ExitCode: 0,
				ParsedOutput: &hook.HookOutput{
					HookSpecificOutput: json.RawMessage(
						`{"hookEventName":"SubagentStart","additionalContext":"Context 3"}`,
					),
				},
			},
		},
	}
	runtime := New(nil, "claude-sonnet-4-5", nil)
	runtime.sessionID = "sess-multi"
	runtime.Hooks = hook.HooksConfig{
		hook.EventSubagentStart: []hook.HookMatcher{{
			Matcher: "general-purpose",
			Hooks: []json.RawMessage{
				json.RawMessage(`{"type":"command","command":"ctx1"}`),
				json.RawMessage(`{"type":"command","command":"guard"}`),
				json.RawMessage(`{"type":"command","command":"ctx3"}`),
			},
		}},
	}
	runtime.HookRunner = hookRunner

	results, blocked, blockingMessages, additionalContext := runtime.RunSubagentStartHooks(
		context.Background(), "agent-multi", "general-purpose", "/repo",
	)

	if !blocked {
		t.Fatalf("blocked = false, want true (one blocking hook present)")
	}
	if len(results) != 3 {
		t.Fatalf("results length = %d, want 3", len(results))
	}
	if len(blockingMessages) != 1 {
		t.Fatalf("blockingMessages length = %d, want 1 (only blocking hook surfaces)", len(blockingMessages))
	}
	if !strings.Contains(blockingMessages[0], "blocked by policy hook") {
		t.Fatalf("blockingMessages[0] = %q, want to contain 'blocked by policy hook'", blockingMessages[0])
	}
	// additionalContext should be empty when blocked.
	if additionalContext != "" {
		t.Fatalf("additionalContext = %q, want empty when blocked", additionalContext)
	}
}

// TestSubagentStartHooksSkippedWhenNoConfig verifies that the runner is not
// invoked when the settings have no SubagentStart hooks configured.
func TestSubagentStartHooksSkippedWhenNoConfig(t *testing.T) {
	hookRunner := &fakeStopHookRunner{}
	runtime := New(nil, "claude-sonnet-4-5", nil)
	runtime.sessionID = "sess-skip"
	runtime.HookRunner = hookRunner

	results, blocked, blockingMessages, additionalContext := runtime.RunSubagentStartHooks(
		context.Background(), "agent-skip", "general-purpose", "",
	)

	if len(hookRunner.calls) != 0 {
		t.Fatalf("call count = %d, want 0 (no hooks configured)", len(hookRunner.calls))
	}
	if blocked {
		t.Fatalf("blocked = true, want false")
	}
	if results != nil || blockingMessages != nil {
		t.Fatalf("results=%v blockingMessages=%v, want both nil", results, blockingMessages)
	}
	if additionalContext != "" {
		t.Fatalf("additionalContext = %q, want empty", additionalContext)
	}
}
