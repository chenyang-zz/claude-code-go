package engine

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/hook"
)

// TestTeammateIdleHookInputType verifies JSON serialization of TeammateIdleHookInput.
func TestTeammateIdleHookInputType(t *testing.T) {
	input := hook.TeammateIdleHookInput{
		BaseHookInput: hook.BaseHookInput{
			SessionID:      "sess-idle",
			TranscriptPath: "/tmp/transcript.jsonl",
			CWD:            "/repo",
		},
		HookEventName: "TeammateIdle",
		TeammateName:  "explore-agent",
		TeamName:      "frontend-team",
	}

	data, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if parsed["hook_event_name"] != "TeammateIdle" {
		t.Fatalf("hook_event_name = %v, want TeammateIdle", parsed["hook_event_name"])
	}
	if parsed["teammate_name"] != "explore-agent" {
		t.Fatalf("teammate_name = %v, want explore-agent", parsed["teammate_name"])
	}
	if parsed["team_name"] != "frontend-team" {
		t.Fatalf("team_name = %v, want frontend-team", parsed["team_name"])
	}
	if parsed["session_id"] != "sess-idle" {
		t.Fatalf("session_id = %v, want sess-idle", parsed["session_id"])
	}
}

// TestRunTeammateIdleHooksDispatched verifies RunTeammateIdleHooks sends the correct input.
func TestRunTeammateIdleHooksDispatched(t *testing.T) {
	hookRunner := &fakeStopHookRunner{
		results: []hook.HookResult{{ExitCode: 0, Stdout: "ok"}},
	}
	runtime := New(nil, "claude-sonnet-4-5", nil)
	runtime.sessionID = "sess-idle"
	runtime.Hooks = hook.HooksConfig{
		hook.EventTeammateIdle: []hook.HookMatcher{{
			Hooks: []json.RawMessage{json.RawMessage(`{"type":"command","command":"echo hi"}`)},
		}},
	}
	runtime.HookRunner = hookRunner

	results, blocked, blockingMessages := runtime.RunTeammateIdleHooks(context.Background(), "explore-agent", "frontend-team", "/workspace")

	if len(hookRunner.calls) != 1 {
		t.Fatalf("call count = %d, want 1", len(hookRunner.calls))
	}
	if blocked {
		t.Fatalf("blocked = true, want false for exit-0 hook")
	}
	if len(blockingMessages) != 0 {
		t.Fatalf("blockingMessages = %v, want empty", blockingMessages)
	}

	input, ok := hookRunner.calls[0].input.(hook.TeammateIdleHookInput)
	if !ok {
		t.Fatalf("input type = %T, want hook.TeammateIdleHookInput", hookRunner.calls[0].input)
	}
	if input.TeammateName != "explore-agent" {
		t.Fatalf("teammate_name = %q, want explore-agent", input.TeammateName)
	}
	if input.HookEventName != "TeammateIdle" {
		t.Fatalf("hook_event_name = %q, want TeammateIdle", input.HookEventName)
	}
	if len(results) != 1 || results[0].ExitCode != 0 {
		t.Fatalf("results = %+v, want one exit-0 result", results)
	}
}

// TestRunTeammateIdleHooksBlocking verifies exit code 2 surfaces as blocked.
func TestRunTeammateIdleHooksBlocking(t *testing.T) {
	hookRunner := &fakeStopHookRunner{
		results: []hook.HookResult{{ExitCode: 2, Stderr: "blocked: idle rejected"}},
	}
	runtime := New(nil, "claude-sonnet-4-5", nil)
	runtime.sessionID = "sess-block"
	runtime.Hooks = hook.HooksConfig{
		hook.EventTeammateIdle: []hook.HookMatcher{{
			Hooks: []json.RawMessage{json.RawMessage(`{"type":"command","command":"deny"}`)},
		}},
	}
	runtime.HookRunner = hookRunner

	results, blocked, blockingMessages := runtime.RunTeammateIdleHooks(context.Background(), "agent", "", "/repo")

	if !blocked {
		t.Fatalf("blocked = false, want true for exit-2 hook")
	}
	if len(results) != 1 {
		t.Fatalf("results length = %d, want 1", len(results))
	}
	if len(blockingMessages) != 1 || !strings.Contains(blockingMessages[0], "blocked") {
		t.Fatalf("blockingMessages = %v, want to contain 'blocked'", blockingMessages)
	}
}

// TestRunTeammateIdleHooksSkippedWhenNoConfig verifies no hooks configured skips dispatch.
func TestRunTeammateIdleHooksSkippedWhenNoConfig(t *testing.T) {
	hookRunner := &fakeStopHookRunner{}
	runtime := New(nil, "claude-sonnet-4-5", nil)
	runtime.sessionID = "sess-skip"
	runtime.HookRunner = hookRunner

	results, blocked, blockingMessages := runtime.RunTeammateIdleHooks(context.Background(), "agent", "", "")

	if len(hookRunner.calls) != 0 {
		t.Fatalf("call count = %d, want 0", len(hookRunner.calls))
	}
	if blocked || results != nil || blockingMessages != nil {
		t.Fatalf("blocked=%v results=%v blockingMessages=%v, want all zero", blocked, results, blockingMessages)
	}
}

// TestConfigChangeHookInputType verifies JSON serialization of ConfigChangeHookInput.
func TestConfigChangeHookInputType(t *testing.T) {
	input := hook.ConfigChangeHookInput{
		BaseHookInput: hook.BaseHookInput{
			SessionID: "sess-cfg",
			CWD:       "/repo",
		},
		HookEventName: "ConfigChange",
		Source:        "project_settings",
		FilePath:      "/repo/.claude/settings.json",
	}

	data, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if parsed["hook_event_name"] != "ConfigChange" {
		t.Fatalf("hook_event_name = %v, want ConfigChange", parsed["hook_event_name"])
	}
	if parsed["source"] != "project_settings" {
		t.Fatalf("source = %v, want project_settings", parsed["source"])
	}
	if parsed["file_path"] != "/repo/.claude/settings.json" {
		t.Fatalf("file_path = %v, want /repo/.claude/settings.json", parsed["file_path"])
	}
}

// TestRunConfigChangeHooksDispatched verifies RunConfigChangeHooks sends the correct input.
func TestRunConfigChangeHooksDispatched(t *testing.T) {
	hookRunner := &fakeStopHookRunner{
		results: []hook.HookResult{{ExitCode: 0, Stdout: "ok"}},
	}
	runtime := New(nil, "claude-sonnet-4-5", nil)
	runtime.sessionID = "sess-cfg"
	runtime.Hooks = hook.HooksConfig{
		hook.EventConfigChange: []hook.HookMatcher{{
			Hooks: []json.RawMessage{json.RawMessage(`{"type":"command","command":"echo hi"}`)},
		}},
	}
	runtime.HookRunner = hookRunner

	results, blocked, blockingMessages := runtime.RunConfigChangeHooks(context.Background(), "project_settings", "/repo/.claude/settings.json", "/workspace")

	if len(hookRunner.calls) != 1 {
		t.Fatalf("call count = %d, want 1", len(hookRunner.calls))
	}
	if blocked {
		t.Fatalf("blocked = true, want false for exit-0 hook")
	}

	input, ok := hookRunner.calls[0].input.(hook.ConfigChangeHookInput)
	if !ok {
		t.Fatalf("input type = %T, want hook.ConfigChangeHookInput", hookRunner.calls[0].input)
	}
	if input.Source != "project_settings" {
		t.Fatalf("source = %q, want project_settings", input.Source)
	}
	if input.FilePath != "/repo/.claude/settings.json" {
		t.Fatalf("file_path = %q, want /repo/.claude/settings.json", input.FilePath)
	}
	if len(results) != 1 || results[0].ExitCode != 0 {
		t.Fatalf("results = %+v, want one exit-0 result", results)
	}
	if len(blockingMessages) != 0 {
		t.Fatalf("blockingMessages = %v, want empty", blockingMessages)
	}
}

// TestRunConfigChangeHooksPolicySettingsBlockingIgnored verifies policy_settings source ignores blocking.
func TestRunConfigChangeHooksPolicySettingsBlockingIgnored(t *testing.T) {
	hookRunner := &fakeStopHookRunner{
		results: []hook.HookResult{{ExitCode: 2, Stderr: "blocked"}},
	}
	runtime := New(nil, "claude-sonnet-4-5", nil)
	runtime.sessionID = "sess-policy"
	runtime.Hooks = hook.HooksConfig{
		hook.EventConfigChange: []hook.HookMatcher{{
			Hooks: []json.RawMessage{json.RawMessage(`{"type":"command","command":"deny"}`)},
		}},
	}
	runtime.HookRunner = hookRunner

	_, blocked, _ := runtime.RunConfigChangeHooks(context.Background(), "policySettings", "/policy.json", "/repo")

	if blocked {
		t.Fatalf("blocked = true, want false for policySettings source")
	}
}

// TestInstructionsLoadedHookInputType verifies JSON serialization of InstructionsLoadedHookInput.
func TestInstructionsLoadedHookInputType(t *testing.T) {
	input := hook.InstructionsLoadedHookInput{
		BaseHookInput: hook.BaseHookInput{
			SessionID: "sess-inst",
			CWD:       "/repo",
		},
		HookEventName:   "InstructionsLoaded",
		FilePath:        "/repo/CLAUDE.md",
		MemoryType:      "Project",
		LoadReason:      "session_start",
		Globs:           []string{"CLAUDE.md"},
		TriggerFilePath: "/repo/src/main.go",
	}

	data, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if parsed["hook_event_name"] != "InstructionsLoaded" {
		t.Fatalf("hook_event_name = %v, want InstructionsLoaded", parsed["hook_event_name"])
	}
	if parsed["file_path"] != "/repo/CLAUDE.md" {
		t.Fatalf("file_path = %v, want /repo/CLAUDE.md", parsed["file_path"])
	}
	if parsed["memory_type"] != "Project" {
		t.Fatalf("memory_type = %v, want Project", parsed["memory_type"])
	}
	if parsed["load_reason"] != "session_start" {
		t.Fatalf("load_reason = %v, want session_start", parsed["load_reason"])
	}
	globs, ok := parsed["globs"].([]any)
	if !ok || len(globs) != 1 || globs[0] != "CLAUDE.md" {
		t.Fatalf("globs = %v, want [CLAUDE.md]", parsed["globs"])
	}
}

// TestRunInstructionsLoadedHooksDispatched verifies RunInstructionsLoadedHooks sends the correct input.
func TestRunInstructionsLoadedHooksDispatched(t *testing.T) {
	hookRunner := &fakeStopHookRunner{
		results: []hook.HookResult{{ExitCode: 0, Stdout: "ok"}},
	}
	runtime := New(nil, "claude-sonnet-4-5", nil)
	runtime.sessionID = "sess-inst"
	runtime.Hooks = hook.HooksConfig{
		hook.EventInstructionsLoaded: []hook.HookMatcher{{
			Hooks: []json.RawMessage{json.RawMessage(`{"type":"command","command":"echo hi"}`)},
		}},
	}
	runtime.HookRunner = hookRunner

	runtime.RunInstructionsLoadedHooks(context.Background(), "/repo/CLAUDE.md", "Project", "session_start", "/workspace", []string{"CLAUDE.md"}, "", "")

	if len(hookRunner.calls) != 1 {
		t.Fatalf("call count = %d, want 1", len(hookRunner.calls))
	}

	input, ok := hookRunner.calls[0].input.(hook.InstructionsLoadedHookInput)
	if !ok {
		t.Fatalf("input type = %T, want hook.InstructionsLoadedHookInput", hookRunner.calls[0].input)
	}
	if input.FilePath != "/repo/CLAUDE.md" {
		t.Fatalf("file_path = %q, want /repo/CLAUDE.md", input.FilePath)
	}
	if input.MemoryType != "Project" {
		t.Fatalf("memory_type = %q, want Project", input.MemoryType)
	}
}

// TestRunInstructionsLoadedHooksSkippedWhenNoConfig verifies no hooks configured skips dispatch.
func TestRunInstructionsLoadedHooksSkippedWhenNoConfig(t *testing.T) {
	hookRunner := &fakeStopHookRunner{}
	runtime := New(nil, "claude-sonnet-4-5", nil)
	runtime.sessionID = "sess-skip"
	runtime.HookRunner = hookRunner

	runtime.RunInstructionsLoadedHooks(context.Background(), "/repo/CLAUDE.md", "Project", "session_start", "", nil, "", "")

	if len(hookRunner.calls) != 0 {
		t.Fatalf("call count = %d, want 0", len(hookRunner.calls))
	}
}

// TestCwdChangedHookInputType verifies JSON serialization of CwdChangedHookInput.
func TestCwdChangedHookInputType(t *testing.T) {
	input := hook.CwdChangedHookInput{
		BaseHookInput: hook.BaseHookInput{
			SessionID: "sess-cwd",
			CWD:       "/repo",
		},
		HookEventName: "CwdChanged",
		OldCWD:        "/repo",
		NewCWD:        "/repo/.claude/worktrees/feat",
	}

	data, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if parsed["hook_event_name"] != "CwdChanged" {
		t.Fatalf("hook_event_name = %v, want CwdChanged", parsed["hook_event_name"])
	}
	if parsed["old_cwd"] != "/repo" {
		t.Fatalf("old_cwd = %v, want /repo", parsed["old_cwd"])
	}
	if parsed["new_cwd"] != "/repo/.claude/worktrees/feat" {
		t.Fatalf("new_cwd = %v, want /repo/.claude/worktrees/feat", parsed["new_cwd"])
	}
}

// TestRunCwdChangedHooksDispatched verifies RunCwdChangedHooks sends the correct input.
func TestRunCwdChangedHooksDispatched(t *testing.T) {
	hookRunner := &fakeStopHookRunner{
		results: []hook.HookResult{{ExitCode: 0, Stdout: "ok"}},
	}
	runtime := New(nil, "claude-sonnet-4-5", nil)
	runtime.sessionID = "sess-cwd"
	runtime.Hooks = hook.HooksConfig{
		hook.EventCwdChanged: []hook.HookMatcher{{
			Hooks: []json.RawMessage{json.RawMessage(`{"type":"command","command":"echo hi"}`)},
		}},
	}
	runtime.HookRunner = hookRunner

	results, blocked, blockingMessages := runtime.RunCwdChangedHooks(context.Background(), "/repo", "/repo/.claude/worktrees/feat", "/workspace")

	if len(hookRunner.calls) != 1 {
		t.Fatalf("call count = %d, want 1", len(hookRunner.calls))
	}
	if blocked {
		t.Fatalf("blocked = true, want false for exit-0 hook")
	}

	input, ok := hookRunner.calls[0].input.(hook.CwdChangedHookInput)
	if !ok {
		t.Fatalf("input type = %T, want hook.CwdChangedHookInput", hookRunner.calls[0].input)
	}
	if input.OldCWD != "/repo" {
		t.Fatalf("old_cwd = %q, want /repo", input.OldCWD)
	}
	if input.NewCWD != "/repo/.claude/worktrees/feat" {
		t.Fatalf("new_cwd = %q, want /repo/.claude/worktrees/feat", input.NewCWD)
	}
	if len(results) != 1 || results[0].ExitCode != 0 {
		t.Fatalf("results = %+v, want one exit-0 result", results)
	}
	if len(blockingMessages) != 0 {
		t.Fatalf("blockingMessages = %v, want empty", blockingMessages)
	}
}

// TestFileChangedHookInputType verifies JSON serialization of FileChangedHookInput.
func TestFileChangedHookInputType(t *testing.T) {
	input := hook.FileChangedHookInput{
		BaseHookInput: hook.BaseHookInput{
			SessionID: "sess-file",
			CWD:       "/repo",
		},
		HookEventName: "FileChanged",
		FilePath:      "/repo/src/main.go",
		Event:         "change",
	}

	data, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if parsed["hook_event_name"] != "FileChanged" {
		t.Fatalf("hook_event_name = %v, want FileChanged", parsed["hook_event_name"])
	}
	if parsed["file_path"] != "/repo/src/main.go" {
		t.Fatalf("file_path = %v, want /repo/src/main.go", parsed["file_path"])
	}
	if parsed["event"] != "change" {
		t.Fatalf("event = %v, want change", parsed["event"])
	}
}

// TestRunFileChangedHooksDispatched verifies RunFileChangedHooks sends the correct input.
func TestRunFileChangedHooksDispatched(t *testing.T) {
	hookRunner := &fakeStopHookRunner{
		results: []hook.HookResult{{ExitCode: 0, Stdout: "ok"}},
	}
	runtime := New(nil, "claude-sonnet-4-5", nil)
	runtime.sessionID = "sess-file"
	runtime.Hooks = hook.HooksConfig{
		hook.EventFileChanged: []hook.HookMatcher{{
			Hooks: []json.RawMessage{json.RawMessage(`{"type":"command","command":"echo hi"}`)},
		}},
	}
	runtime.HookRunner = hookRunner

	results, blocked, blockingMessages := runtime.RunFileChangedHooks(context.Background(), "/repo/src/main.go", "change", "/workspace")

	if len(hookRunner.calls) != 1 {
		t.Fatalf("call count = %d, want 1", len(hookRunner.calls))
	}
	if blocked {
		t.Fatalf("blocked = true, want false for exit-0 hook")
	}

	input, ok := hookRunner.calls[0].input.(hook.FileChangedHookInput)
	if !ok {
		t.Fatalf("input type = %T, want hook.FileChangedHookInput", hookRunner.calls[0].input)
	}
	if input.FilePath != "/repo/src/main.go" {
		t.Fatalf("file_path = %q, want /repo/src/main.go", input.FilePath)
	}
	if input.Event != "change" {
		t.Fatalf("event = %q, want change", input.Event)
	}
	if len(results) != 1 || results[0].ExitCode != 0 {
		t.Fatalf("results = %+v, want one exit-0 result", results)
	}
	if len(blockingMessages) != 0 {
		t.Fatalf("blockingMessages = %v, want empty", blockingMessages)
	}
}

// TestPermissionRequestHookInputType verifies JSON serialization of PermissionRequestHookInput.
func TestPermissionRequestHookInputType(t *testing.T) {
	input := hook.PermissionRequestHookInput{
		BaseHookInput: hook.BaseHookInput{
			SessionID: "sess-perm",
			CWD:       "/repo",
		},
		HookEventName: "PermissionRequest",
		ToolName:      "Bash",
		ToolInput:     json.RawMessage(`{"command":"ls"}`),
	}

	data, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if parsed["hook_event_name"] != "PermissionRequest" {
		t.Fatalf("hook_event_name = %v, want PermissionRequest", parsed["hook_event_name"])
	}
	if parsed["tool_name"] != "Bash" {
		t.Fatalf("tool_name = %v, want Bash", parsed["tool_name"])
	}
	toolInput, ok := parsed["tool_input"].(map[string]any)
	if !ok || toolInput["command"] != "ls" {
		t.Fatalf("tool_input = %v, want command:ls", parsed["tool_input"])
	}
}

// TestRunPermissionRequestHooksDispatched verifies RunPermissionRequestHooks sends the correct input.
func TestRunPermissionRequestHooksDispatched(t *testing.T) {
	hookRunner := &fakeStopHookRunner{
		results: []hook.HookResult{{ExitCode: 0, Stdout: "ok"}},
	}
	runtime := New(nil, "claude-sonnet-4-5", nil)
	runtime.sessionID = "sess-perm"
	runtime.Hooks = hook.HooksConfig{
		hook.EventPermissionRequest: []hook.HookMatcher{{
			Hooks: []json.RawMessage{json.RawMessage(`{"type":"command","command":"echo hi"}`)},
		}},
	}
	runtime.HookRunner = hookRunner

	results, blocked, blockingMessages := runtime.RunPermissionRequestHooks(context.Background(), "Bash", json.RawMessage(`{"command":"ls"}`), "/workspace")

	if len(hookRunner.calls) != 1 {
		t.Fatalf("call count = %d, want 1", len(hookRunner.calls))
	}
	if blocked {
		t.Fatalf("blocked = true, want false for exit-0 hook")
	}

	input, ok := hookRunner.calls[0].input.(hook.PermissionRequestHookInput)
	if !ok {
		t.Fatalf("input type = %T, want hook.PermissionRequestHookInput", hookRunner.calls[0].input)
	}
	if input.ToolName != "Bash" {
		t.Fatalf("tool_name = %q, want Bash", input.ToolName)
	}
	if len(results) != 1 || results[0].ExitCode != 0 {
		t.Fatalf("results = %+v, want one exit-0 result", results)
	}
	if len(blockingMessages) != 0 {
		t.Fatalf("blockingMessages = %v, want empty", blockingMessages)
	}
}

// TestPermissionDeniedHookInputType verifies JSON serialization of PermissionDeniedHookInput.
func TestPermissionDeniedHookInputType(t *testing.T) {
	input := hook.PermissionDeniedHookInput{
		BaseHookInput: hook.BaseHookInput{
			SessionID: "sess-deny",
			CWD:       "/repo",
		},
		HookEventName: "PermissionDenied",
		ToolName:      "Bash",
		ToolInput:     json.RawMessage(`{"command":"rm -rf /"}`),
		ToolUseID:     "toolu_123",
		Reason:        "Permission to execute rm -rf / was not granted.",
	}

	data, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if parsed["hook_event_name"] != "PermissionDenied" {
		t.Fatalf("hook_event_name = %v, want PermissionDenied", parsed["hook_event_name"])
	}
	if parsed["tool_name"] != "Bash" {
		t.Fatalf("tool_name = %v, want Bash", parsed["tool_name"])
	}
	if parsed["tool_use_id"] != "toolu_123" {
		t.Fatalf("tool_use_id = %v, want toolu_123", parsed["tool_use_id"])
	}
	if parsed["reason"] != "Permission to execute rm -rf / was not granted." {
		t.Fatalf("reason = %v, want Permission to execute rm -rf / was not granted.", parsed["reason"])
	}
}

// TestRunPermissionDeniedHooksDispatched verifies RunPermissionDeniedHooks sends the correct input.
func TestRunPermissionDeniedHooksDispatched(t *testing.T) {
	hookRunner := &fakeStopHookRunner{
		results: []hook.HookResult{{ExitCode: 0, Stdout: "ok"}},
	}
	runtime := New(nil, "claude-sonnet-4-5", nil)
	runtime.sessionID = "sess-deny"
	runtime.Hooks = hook.HooksConfig{
		hook.EventPermissionDenied: []hook.HookMatcher{{
			Hooks: []json.RawMessage{json.RawMessage(`{"type":"command","command":"echo hi"}`)},
		}},
	}
	runtime.HookRunner = hookRunner

	results, blocked, blockingMessages := runtime.RunPermissionDeniedHooks(context.Background(), "Bash", json.RawMessage(`{"command":"rm -rf /"}`), "toolu_123", "Permission denied", "/workspace")

	if len(hookRunner.calls) != 1 {
		t.Fatalf("call count = %d, want 1", len(hookRunner.calls))
	}
	if blocked {
		t.Fatalf("blocked = true, want false for exit-0 hook")
	}

	input, ok := hookRunner.calls[0].input.(hook.PermissionDeniedHookInput)
	if !ok {
		t.Fatalf("input type = %T, want hook.PermissionDeniedHookInput", hookRunner.calls[0].input)
	}
	if input.ToolName != "Bash" {
		t.Fatalf("tool_name = %q, want Bash", input.ToolName)
	}
	if input.ToolUseID != "toolu_123" {
		t.Fatalf("tool_use_id = %q, want toolu_123", input.ToolUseID)
	}
	if input.Reason != "Permission denied" {
		t.Fatalf("reason = %q, want Permission denied", input.Reason)
	}
	if len(results) != 1 || results[0].ExitCode != 0 {
		t.Fatalf("results = %+v, want one exit-0 result", results)
	}
	if len(blockingMessages) != 0 {
		t.Fatalf("blockingMessages = %v, want empty", blockingMessages)
	}
}

// TestPermissionRequestHooksBlocking verifies blocking for PermissionRequest.
func TestPermissionRequestHooksBlocking(t *testing.T) {
	hookRunner := &fakeStopHookRunner{
		results: []hook.HookResult{{ExitCode: 2, Stderr: "blocked: dangerous command"}},
	}
	runtime := New(nil, "claude-sonnet-4-5", nil)
	runtime.sessionID = "sess-block"
	runtime.Hooks = hook.HooksConfig{
		hook.EventPermissionRequest: []hook.HookMatcher{{
			Hooks: []json.RawMessage{json.RawMessage(`{"type":"command","command":"deny"}`)},
		}},
	}
	runtime.HookRunner = hookRunner

	results, blocked, blockingMessages := runtime.RunPermissionRequestHooks(context.Background(), "Bash", json.RawMessage(`{"command":"rm -rf /"}`), "/repo")

	if !blocked {
		t.Fatalf("blocked = false, want true for exit-2 hook")
	}
	if len(results) != 1 {
		t.Fatalf("results length = %d, want 1", len(results))
	}
	if len(blockingMessages) != 1 || !strings.Contains(blockingMessages[0], "dangerous command") {
		t.Fatalf("blockingMessages = %v, want to contain 'dangerous command'", blockingMessages)
	}
}

// TestAllStatusPermissionHooksSkippedWhenDisabled verifies DisableAllHooks short-circuits all new events.
func TestAllStatusPermissionHooksSkippedWhenDisabled(t *testing.T) {
	tests := []struct {
		name string
		run  func(*Runtime)
	}{
		{
			name: "TeammateIdle",
			run: func(r *Runtime) {
				r.RunTeammateIdleHooks(context.Background(), "agent", "", "/repo")
			},
		},
		{
			name: "ConfigChange",
			run: func(r *Runtime) {
				r.RunConfigChangeHooks(context.Background(), "user_settings", "", "/repo")
			},
		},
		{
			name: "InstructionsLoaded",
			run: func(r *Runtime) {
				r.RunInstructionsLoadedHooks(context.Background(), "/repo/CLAUDE.md", "Project", "session_start", "/repo", nil, "", "")
			},
		},
		{
			name: "CwdChanged",
			run: func(r *Runtime) {
				r.RunCwdChangedHooks(context.Background(), "/repo", "/repo2", "/repo")
			},
		},
		{
			name: "FileChanged",
			run: func(r *Runtime) {
				r.RunFileChangedHooks(context.Background(), "/repo/src/main.go", "change", "/repo")
			},
		},
		{
			name: "PermissionRequest",
			run: func(r *Runtime) {
				r.RunPermissionRequestHooks(context.Background(), "Bash", json.RawMessage(`{}`), "/repo")
			},
		},
		{
			name: "PermissionDenied",
			run: func(r *Runtime) {
				r.RunPermissionDeniedHooks(context.Background(), "Bash", json.RawMessage(`{}`), "id", "denied", "/repo")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hookRunner := &fakeStopHookRunner{}
			runtime := New(nil, "claude-sonnet-4-5", nil)
			runtime.sessionID = "sess-disabled"
			runtime.DisableAllHooks = true
			runtime.Hooks = hook.HooksConfig{
				hook.EventTeammateIdle:       []hook.HookMatcher{{Hooks: []json.RawMessage{json.RawMessage(`{"type":"command","command":"deny"}`)}}},
				hook.EventConfigChange:       []hook.HookMatcher{{Hooks: []json.RawMessage{json.RawMessage(`{"type":"command","command":"deny"}`)}}},
				hook.EventInstructionsLoaded: []hook.HookMatcher{{Hooks: []json.RawMessage{json.RawMessage(`{"type":"command","command":"deny"}`)}}},
				hook.EventCwdChanged:         []hook.HookMatcher{{Hooks: []json.RawMessage{json.RawMessage(`{"type":"command","command":"deny"}`)}}},
				hook.EventFileChanged:        []hook.HookMatcher{{Hooks: []json.RawMessage{json.RawMessage(`{"type":"command","command":"deny"}`)}}},
				hook.EventPermissionRequest:  []hook.HookMatcher{{Hooks: []json.RawMessage{json.RawMessage(`{"type":"command","command":"deny"}`)}}},
				hook.EventPermissionDenied:   []hook.HookMatcher{{Hooks: []json.RawMessage{json.RawMessage(`{"type":"command","command":"deny"}`)}}},
			}
			runtime.HookRunner = hookRunner

			tt.run(runtime)

			if len(hookRunner.calls) != 0 {
				t.Fatalf("call count = %d, want 0 (hooks globally disabled)", len(hookRunner.calls))
			}
		})
	}
}
