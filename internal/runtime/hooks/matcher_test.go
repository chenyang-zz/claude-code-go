package hooks

import (
	"encoding/json"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/hook"
)

func TestMatchHooks_StopEvent(t *testing.T) {
	config := hook.HooksConfig{
		hook.EventStop: []hook.HookMatcher{
			{
				Hooks: []json.RawMessage{
					json.RawMessage(`{"type":"command","command":"stop-handler.sh"}`),
				},
			},
		},
	}

	results := MatchHooks(config, hook.EventStop, MatchQuery{})
	if len(results) != 1 {
		t.Fatalf("expected 1 hook, got %d", len(results))
	}
	if results[0].Command != "stop-handler.sh" {
		t.Errorf("expected stop-handler.sh, got %s", results[0].Command)
	}
}

func TestMatchHooks_EmptyConfig(t *testing.T) {
	results := MatchHooks(nil, hook.EventStop, MatchQuery{})
	if len(results) != 0 {
		t.Errorf("expected 0 hooks, got %d", len(results))
	}
}

func TestMatchHooks_WithMatcher(t *testing.T) {
	config := hook.HooksConfig{
		hook.EventPreToolUse: []hook.HookMatcher{
			{
				Matcher: "Bash",
				Hooks: []json.RawMessage{
					json.RawMessage(`{"type":"command","command":"check-bash.sh"}`),
				},
			},
			{
				Matcher: "Write",
				Hooks: []json.RawMessage{
					json.RawMessage(`{"type":"command","command":"check-write.sh"}`),
				},
			},
		},
	}

	// Match Bash tool specifically
	results := MatchHooks(config, hook.EventPreToolUse, MatchQuery{ToolName: "Bash"})
	if len(results) != 1 {
		t.Fatalf("expected 1 hook for Bash, got %d", len(results))
	}
	if results[0].Command != "check-bash.sh" {
		t.Errorf("expected check-bash.sh, got %s", results[0].Command)
	}

	// Empty matcher matches everything
	config2 := hook.HooksConfig{
		hook.EventPreToolUse: []hook.HookMatcher{
			{
				Hooks: []json.RawMessage{
					json.RawMessage(`{"type":"command","command":"all-tools.sh"}`),
				},
			},
		},
	}
	results2 := MatchHooks(config2, hook.EventPreToolUse, MatchQuery{ToolName: "Bash"})
	if len(results2) != 1 {
		t.Errorf("expected 1 hook for empty matcher, got %d", len(results2))
	}
}

func TestMatchHooks_SkipsNonCommandHooks(t *testing.T) {
	config := hook.HooksConfig{
		hook.EventStop: []hook.HookMatcher{
			{
				Hooks: []json.RawMessage{
					json.RawMessage(`{"type":"prompt","prompt":"test"}`),
					json.RawMessage(`{"type":"command","command":"valid.sh"}`),
					json.RawMessage(`invalid json`),
				},
			},
		},
	}

	results := MatchHooks(config, hook.EventStop, MatchQuery{})
	if len(results) != 1 {
		t.Errorf("expected 1 command hook, got %d", len(results))
	}
	if results[0].Command != "valid.sh" {
		t.Errorf("expected valid.sh, got %s", results[0].Command)
	}
}
