package hook

import (
	"encoding/json"
	"fmt"
)

// HookType identifies the concrete hook implementation variant.
type HookType string

const (
	// TypeCommand executes a shell command with JSON stdin.
	TypeCommand HookType = "command"
	// TypePrompt sends an LLM prompt (not yet migrated).
	TypePrompt HookType = "prompt"
	// TypeHTTP sends an HTTP request (not yet migrated).
	TypeHTTP HookType = "http"
	// TypeAgent spawns an agent sub-task (not yet migrated).
	TypeAgent HookType = "agent"
)

// CommandHook stores a command-type hook configuration that executes a shell command.
type CommandHook struct {
	// Type is always "command".
	Type HookType `json:"type"`
	// Command is the shell command to execute.
	Command string `json:"command"`
	// If is an optional permission-rule-syntax filter (e.g. "Bash(git *)").
	If string `json:"if,omitempty"`
	// Shell selects the shell provider; defaults to "bash" when empty.
	Shell string `json:"shell,omitempty"`
	// Timeout is the per-hook timeout in seconds. Zero uses the default (600s).
	Timeout int `json:"timeout,omitempty"`
	// StatusMessage is an optional custom spinner message shown during execution.
	StatusMessage string `json:"statusMessage,omitempty"`
	// Once removes the hook after its first execution when true.
	Once bool `json:"once,omitempty"`
	// Async runs the hook in the background without blocking the conversation.
	Async bool `json:"async,omitempty"`
	// AsyncRewake runs in background but re-wakes the model on exit code 2.
	AsyncRewake bool `json:"asyncRewake,omitempty"`
}

// HookMatcher groups one or more hooks under an optional matcher pattern.
type HookMatcher struct {
	// Matcher is an optional string pattern used to filter hooks (e.g. tool name "Write").
	Matcher string `json:"matcher,omitempty"`
	// Hooks lists the hook commands to execute when the matcher matches.
	Hooks []json.RawMessage `json:"hooks"`
}

// HooksConfig maps hook events to their configured matcher groups.
// The JSON representation is a partial record keyed by event name.
type HooksConfig map[HookEvent][]HookMatcher

// CommandHooks extracts all command-type hooks for the given event.
func (c HooksConfig) CommandHooks(event HookEvent) []CommandHook {
	matchers, ok := c[event]
	if !ok {
		return nil
	}
	var result []CommandHook
	for _, m := range matchers {
		for _, raw := range m.Hooks {
			var peek struct {
				Type HookType `json:"type"`
			}
			if err := json.Unmarshal(raw, &peek); err != nil {
				continue
			}
			if peek.Type != TypeCommand {
				continue
			}
			var cmd CommandHook
			if err := json.Unmarshal(raw, &cmd); err != nil {
				continue
			}
			result = append(result, cmd)
		}
	}
	return result
}

// HasEvent reports whether at least one hook is configured for the given event.
func (c HooksConfig) HasEvent(event HookEvent) bool {
	matchers, ok := c[event]
	if !ok {
		return false
	}
	for _, m := range matchers {
		if len(m.Hooks) > 0 {
			return true
		}
	}
	return false
}

// ParseHooksConfig decodes a raw JSON object into a HooksConfig.
// Unknown event keys are silently ignored.
func ParseHooksConfig(raw map[string]json.RawMessage) (HooksConfig, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	cfg := make(HooksConfig, len(raw))
	for key, value := range raw {
		event := HookEvent(key)
		if !event.IsValid() {
			continue
		}
		var matchers []HookMatcher
		if err := json.Unmarshal(value, &matchers); err != nil {
			return nil, fmt.Errorf("parse hooks for event %s: %w", key, err)
		}
		cfg[event] = matchers
	}
	if len(cfg) == 0 {
		return nil, nil
	}
	return cfg, nil
}

// MergeHooksConfig overlays the override hooks on top of the base.
// Override events replace base events entirely (no deep merge of matchers).
func MergeHooksConfig(base, override HooksConfig) HooksConfig {
	if len(base) == 0 && len(override) == 0 {
		return nil
	}
	merged := make(HooksConfig, len(base)+len(override))
	for k, v := range base {
		merged[k] = v
	}
	for k, v := range override {
		merged[k] = v
	}
	if len(merged) == 0 {
		return nil
	}
	return merged
}
