package hooks

import (
	"encoding/json"

	"github.com/sheepzhao/claude-code-go/internal/core/hook"
)

// MatchQuery carries the optional query parameters used to filter hooks by matcher pattern.
// For stop-type events (Stop, SubagentStop, StopFailure), all fields are empty and every
// configured hook matches regardless of the matcher pattern.
type MatchQuery struct {
	// ToolName is the tool name used to match against HookMatcher.Matcher (e.g. "Bash", "Write").
	ToolName string
	// Matcher is a generic match key used for non-tool hook families such as MCP server names.
	Matcher string
}

// MatchHooks returns all command hooks that match the given event and query.
// For stop-type events, the query is ignored and all hooks for the event are returned.
func MatchHooks(config hook.HooksConfig, event hook.HookEvent, query MatchQuery) []hook.CommandHook {
	matchers, ok := config[event]
	if !ok {
		return nil
	}

	var result []hook.CommandHook
	for _, m := range matchers {
		if !matchesMatcherPattern(m.Matcher, query) {
			continue
		}
		for _, raw := range m.Hooks {
			cmdHook, ok := decodeCommandHook(raw)
			if !ok {
				continue
			}
			if !matchesIfCondition(cmdHook.If, query) {
				continue
			}
			result = append(result, cmdHook)
		}
	}
	return result
}

// matchesMatcherPattern checks whether a matcher pattern matches the query.
// An empty pattern matches everything.
func matchesMatcherPattern(pattern string, query MatchQuery) bool {
	if pattern == "" {
		return true
	}
	if query.Matcher != "" {
		return pattern == query.Matcher
	}
	// Stop-type events have no tool name query, so all patterns match.
	if query.ToolName == "" {
		return true
	}
	return pattern == query.ToolName
}

// matchesIfCondition evaluates a hook's "if" condition against the match query.
// The "if" field uses permission rule syntax (e.g. "Bash(git *)").
// Minimal implementation for stop events: empty condition always matches.
// Full permission rule parsing is deferred to future batches when Pre/Post-tool hooks are added.
func matchesIfCondition(condition string, query MatchQuery) bool {
	if condition == "" {
		return true
	}
	// Stop-type events have no tool context, so "if" conditions are not applicable.
	if query.ToolName == "" {
		return true
	}
	// Future: parse permission rule syntax like "Bash(git *)" for tool events.
	return true
}

// decodeCommandHook decodes a raw JSON message as a command-type hook.
// Returns the hook and true if it is a valid command hook; otherwise false.
func decodeCommandHook(raw json.RawMessage) (hook.CommandHook, bool) {
	var peek struct {
		Type hook.HookType `json:"type"`
	}
	if err := json.Unmarshal(raw, &peek); err != nil {
		return hook.CommandHook{}, false
	}
	if peek.Type != hook.TypeCommand {
		return hook.CommandHook{}, false
	}
	var cmd hook.CommandHook
	if err := json.Unmarshal(raw, &cmd); err != nil {
		return hook.CommandHook{}, false
	}
	return cmd, true
}
