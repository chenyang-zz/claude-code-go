package engine

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/hook"
)

// TestUserPromptSubmitHookInputType verifies the JSON serialization of the
// new UserPromptSubmitHookInput matches the schema expected by hook commands
// (hook_event_name="UserPromptSubmit", prompt body, BaseHookInput fields).
func TestUserPromptSubmitHookInputType(t *testing.T) {
	input := hook.UserPromptSubmitHookInput{
		BaseHookInput: hook.BaseHookInput{
			SessionID:      "sess-prompt",
			TranscriptPath: "/tmp/transcript.jsonl",
			CWD:            "/repo",
		},
		HookEventName: "UserPromptSubmit",
		Prompt:        "explain the architecture",
	}

	data, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if parsed["hook_event_name"] != "UserPromptSubmit" {
		t.Fatalf("hook_event_name = %v, want UserPromptSubmit", parsed["hook_event_name"])
	}
	if parsed["prompt"] != "explain the architecture" {
		t.Fatalf("prompt = %v, want 'explain the architecture'", parsed["prompt"])
	}
	if parsed["session_id"] != "sess-prompt" {
		t.Fatalf("session_id = %v, want sess-prompt", parsed["session_id"])
	}
	if parsed["transcript_path"] != "/tmp/transcript.jsonl" {
		t.Fatalf("transcript_path = %v, want /tmp/transcript.jsonl", parsed["transcript_path"])
	}
	if parsed["cwd"] != "/repo" {
		t.Fatalf("cwd = %v, want /repo", parsed["cwd"])
	}
}

// TestUserPromptSubmitHooksDispatched verifies that RunUserPromptSubmitHooks
// sends a hook input through the runner with the matching event name and
// prompt body when the user prompt is forwarded to the model.
func TestUserPromptSubmitHooksDispatched(t *testing.T) {
	hookRunner := &fakeStopHookRunner{
		results: []hook.HookResult{{ExitCode: 0, Stdout: "ok"}},
	}
	runtime := New(nil, "claude-sonnet-4-5", nil)
	runtime.TranscriptPath = "/tmp/transcript.jsonl"
	runtime.sessionID = "sess-prompt"
	runtime.Hooks = hook.HooksConfig{
		hook.EventUserPromptSubmit: []hook.HookMatcher{{
			Hooks: []json.RawMessage{json.RawMessage(`{"type":"command","command":"echo hi"}`)},
		}},
	}
	runtime.HookRunner = hookRunner

	results, blocked, blockingMessages := runtime.RunUserPromptSubmitHooks(context.Background(), "ping", "/workspace")

	if len(hookRunner.calls) != 1 {
		t.Fatalf("call count = %d, want 1", len(hookRunner.calls))
	}
	if blocked {
		t.Fatalf("blocked = true, want false for exit-0 hook")
	}
	if len(blockingMessages) != 0 {
		t.Fatalf("blockingMessages = %v, want empty when not blocked", blockingMessages)
	}
	if len(results) != 1 || results[0].ExitCode != 0 {
		t.Fatalf("results = %+v, want one exit-0 result", results)
	}

	input, ok := hookRunner.calls[0].input.(hook.UserPromptSubmitHookInput)
	if !ok {
		t.Fatalf("input type = %T, want hook.UserPromptSubmitHookInput", hookRunner.calls[0].input)
	}
	if input.Prompt != "ping" {
		t.Fatalf("prompt = %q, want 'ping'", input.Prompt)
	}
	if input.HookEventName != "UserPromptSubmit" {
		t.Fatalf("hook_event_name = %q, want UserPromptSubmit", input.HookEventName)
	}
	if input.CWD != "/workspace" {
		t.Fatalf("cwd = %q, want /workspace", input.CWD)
	}
}

// TestUserPromptSubmitHooksSkippedWhenNoConfig verifies that the runner is
// not invoked when the settings have no UserPromptSubmit hooks configured.
func TestUserPromptSubmitHooksSkippedWhenNoConfig(t *testing.T) {
	hookRunner := &fakeStopHookRunner{}
	runtime := New(nil, "claude-sonnet-4-5", nil)
	runtime.sessionID = "sess-skip"
	runtime.HookRunner = hookRunner

	results, blocked, blockingMessages := runtime.RunUserPromptSubmitHooks(context.Background(), "noop", "")

	if len(hookRunner.calls) != 0 {
		t.Fatalf("call count = %d, want 0 (no hooks configured)", len(hookRunner.calls))
	}
	if blocked {
		t.Fatalf("blocked = true, want false when no hooks configured")
	}
	if results != nil {
		t.Fatalf("results = %v, want nil", results)
	}
	if blockingMessages != nil {
		t.Fatalf("blockingMessages = %v, want nil", blockingMessages)
	}
}

// TestUserPromptSubmitHooksBlocking verifies that exit code 2 from a hook
// surfaces as blocked=true and the stderr is collected in blockingMessages.
func TestUserPromptSubmitHooksBlocking(t *testing.T) {
	hookRunner := &fakeStopHookRunner{
		results: []hook.HookResult{
			{ExitCode: 2, Stderr: "policy violation: reject 'ping'"},
		},
	}
	runtime := New(nil, "claude-sonnet-4-5", nil)
	runtime.sessionID = "sess-block"
	runtime.Hooks = hook.HooksConfig{
		hook.EventUserPromptSubmit: []hook.HookMatcher{{
			Hooks: []json.RawMessage{json.RawMessage(`{"type":"command","command":"deny"}`)},
		}},
	}
	runtime.HookRunner = hookRunner

	results, blocked, blockingMessages := runtime.RunUserPromptSubmitHooks(context.Background(), "ping", "/repo")

	if !blocked {
		t.Fatalf("blocked = false, want true for exit-2 hook")
	}
	if len(results) != 1 {
		t.Fatalf("results length = %d, want 1", len(results))
	}
	if len(blockingMessages) != 1 {
		t.Fatalf("blockingMessages length = %d, want 1", len(blockingMessages))
	}
	if !strings.Contains(blockingMessages[0], "policy violation") {
		t.Fatalf("blockingMessages[0] = %q, want to contain 'policy violation'", blockingMessages[0])
	}
}

// TestUserPromptSubmitHooksMultipleResults verifies that when multiple hooks
// are configured, blocking and non-blocking exits are interleaved and only
// the blocking stderr is surfaced in blockingMessages.
func TestUserPromptSubmitHooksMultipleResults(t *testing.T) {
	hookRunner := &fakeStopHookRunner{
		results: []hook.HookResult{
			{ExitCode: 0, Stdout: "ok"},
			{ExitCode: 2, Stderr: "blocked by guard"},
			{ExitCode: 0, Stdout: "logged"},
		},
	}
	runtime := New(nil, "claude-sonnet-4-5", nil)
	runtime.sessionID = "sess-multi"
	runtime.Hooks = hook.HooksConfig{
		hook.EventUserPromptSubmit: []hook.HookMatcher{{
			Hooks: []json.RawMessage{
				json.RawMessage(`{"type":"command","command":"audit"}`),
				json.RawMessage(`{"type":"command","command":"guard"}`),
				json.RawMessage(`{"type":"command","command":"log"}`),
			},
		}},
	}
	runtime.HookRunner = hookRunner

	results, blocked, blockingMessages := runtime.RunUserPromptSubmitHooks(context.Background(), "ping", "/repo")

	if !blocked {
		t.Fatalf("blocked = false, want true (one blocking hook present)")
	}
	if len(results) != 3 {
		t.Fatalf("results length = %d, want 3", len(results))
	}
	if len(blockingMessages) != 1 {
		t.Fatalf("blockingMessages length = %d, want 1 (only blocking hook surfaces)", len(blockingMessages))
	}
	if !strings.Contains(blockingMessages[0], "blocked by guard") {
		t.Fatalf("blockingMessages[0] = %q, want to contain 'blocked by guard'", blockingMessages[0])
	}
}

// TestUserPromptSubmitHooksSkippedWhenDisabled verifies the global
// DisableAllHooks flag short-circuits hook dispatch even when matchers exist.
func TestUserPromptSubmitHooksSkippedWhenDisabled(t *testing.T) {
	hookRunner := &fakeStopHookRunner{}
	runtime := New(nil, "claude-sonnet-4-5", nil)
	runtime.sessionID = "sess-disabled"
	runtime.DisableAllHooks = true
	runtime.Hooks = hook.HooksConfig{
		hook.EventUserPromptSubmit: []hook.HookMatcher{{
			Hooks: []json.RawMessage{json.RawMessage(`{"type":"command","command":"deny"}`)},
		}},
	}
	runtime.HookRunner = hookRunner

	results, blocked, blockingMessages := runtime.RunUserPromptSubmitHooks(context.Background(), "ping", "/repo")

	if len(hookRunner.calls) != 0 {
		t.Fatalf("call count = %d, want 0 (hooks globally disabled)", len(hookRunner.calls))
	}
	if blocked {
		t.Fatalf("blocked = true, want false when hooks disabled")
	}
	if results != nil || blockingMessages != nil {
		t.Fatalf("results=%v blockingMessages=%v, want both nil when disabled", results, blockingMessages)
	}
}
