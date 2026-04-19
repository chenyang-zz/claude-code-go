package hook

import (
	"encoding/json"
	"testing"
)

func TestHookEventIsValid(t *testing.T) {
	tests := []struct {
		event HookEvent
		valid bool
	}{
		{EventStop, true},
		{EventSubagentStop, true},
		{EventStopFailure, true},
		{EventPreToolUse, true},
		{HookEvent("Unknown"), false},
		{HookEvent(""), false},
	}
	for _, tt := range tests {
		if got := tt.event.IsValid(); got != tt.valid {
			t.Errorf("HookEvent(%q).IsValid() = %v, want %v", tt.event, got, tt.valid)
		}
	}
}

func TestParseHooksConfig(t *testing.T) {
	raw := map[string]json.RawMessage{
		"Stop": json.RawMessage(`[{"hooks":[{"type":"command","command":"echo hello"}]}]`),
		"Unknown": json.RawMessage(`[{"hooks":[]}]`),
	}
	cfg, err := ParseHooksConfig(raw)
	if err != nil {
		t.Fatalf("ParseHooksConfig: %v", err)
	}
	if !cfg.HasEvent(EventStop) {
		t.Error("expected Stop event in config")
	}
	cmds := cfg.CommandHooks(EventStop)
	if len(cmds) != 1 || cmds[0].Command != "echo hello" {
		t.Errorf("CommandHooks(Stop) = %v, want 1 command with 'echo hello'", cmds)
	}
}

func TestParseHooksConfigEmpty(t *testing.T) {
	cfg, err := ParseHooksConfig(nil)
	if err != nil {
		t.Fatalf("ParseHooksConfig(nil): %v", err)
	}
	if cfg != nil {
		t.Error("expected nil for empty input")
	}
}

func TestMergeHooksConfig(t *testing.T) {
	base := HooksConfig{
		EventStop: []HookMatcher{{Hooks: []json.RawMessage{json.RawMessage(`{"type":"command","command":"base"}`)}}},
	}
	override := HooksConfig{
		EventStop:       []HookMatcher{{Hooks: []json.RawMessage{json.RawMessage(`{"type":"command","command":"override"}`)}}},
		EventSessionStart: []HookMatcher{{Hooks: []json.RawMessage{json.RawMessage(`{"type":"command","command":"session"}`)}}},
	}
	merged := MergeHooksConfig(base, override)
	if len(merged[EventStop]) != 1 {
		t.Errorf("expected override to replace Stop, got %d matchers", len(merged[EventStop]))
	}
	if !merged.HasEvent(EventSessionStart) {
		t.Error("expected SessionStart from override")
	}
}

func TestHookResultPredicates(t *testing.T) {
	tests := []struct {
		result    HookResult
		success   bool
		blocking  bool
		isErr     bool
	}{
		{HookResult{ExitCode: 0}, true, false, false},
		{HookResult{ExitCode: 2, Stderr: "blocked"}, false, true, false},
		{HookResult{ExitCode: 1, Stderr: "failed"}, false, false, true},
		{HookResult{ExitCode: 127, Stderr: "not found"}, false, false, true},
	}
	for _, tt := range tests {
		if got := tt.result.IsSuccess(); got != tt.success {
			t.Errorf("IsSuccess(%d) = %v, want %v", tt.result.ExitCode, got, tt.success)
		}
		if got := tt.result.IsBlocking(); got != tt.blocking {
			t.Errorf("IsBlocking(%d) = %v, want %v", tt.result.ExitCode, got, tt.blocking)
		}
		if got := tt.result.IsError(); got != tt.isErr {
			t.Errorf("IsError(%d) = %v, want %v", tt.result.ExitCode, got, tt.isErr)
		}
	}
}
