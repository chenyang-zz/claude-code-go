package plugin

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Known hook event names as defined in the Claude Code hook system.
// These match the 27 HookEvent values from the TypeScript source.
var HookEventNames = []string{
	"PreToolUse",
	"PostToolUse",
	"PostToolUseFailure",
	"Notification",
	"UserPromptSubmit",
	"SessionStart",
	"SessionEnd",
	"Stop",
	"StopFailure",
	"SubagentStart",
	"SubagentStop",
	"PreCompact",
	"PostCompact",
	"PermissionRequest",
	"PermissionDenied",
	"Setup",
	"TeammateIdle",
	"TaskCreated",
	"TaskCompleted",
	"Elicitation",
	"ElicitationResult",
	"ConfigChange",
	"WorktreeCreate",
	"WorktreeRemove",
	"InstructionsLoaded",
	"CwdChanged",
	"FileChanged",
}

// LoadHooksConfig reads and parses the hooks/hooks.json file from the given
// plugin directory. It returns nil, nil if the file does not exist; an error
// is returned only if the file exists but cannot be read or parsed.
func LoadHooksConfig(pluginPath string) (*HooksConfig, error) {
	hooksPath := filepath.Join(pluginPath, "hooks", "hooks.json")
	data, err := os.ReadFile(hooksPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read hooks config %s: %w", hooksPath, err)
	}

	var config HooksConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse hooks config %s: %w", hooksPath, err)
	}

	return &config, nil
}

// ExtractHooks converts the plugin's HooksConfig into a map of hook event
// names to PluginHookMatcher slices, enriched with plugin context information
// (PluginRoot, PluginName, PluginID).
//
// All 27 known hook event names are initialized with empty slices, ensuring
// a complete event map is returned even if the plugin has no hooks configured.
func ExtractHooks(plugin *LoadedPlugin) map[string][]PluginHookMatcher {
	result := make(map[string][]PluginHookMatcher, len(HookEventNames))

	// Initialize all known hook events with empty slices.
	for _, event := range HookEventNames {
		result[event] = nil
	}

	if plugin.HooksConfig == nil || len(plugin.HooksConfig.Events) == 0 {
		return result
	}

	sourceID := plugin.Name + "@" + plugin.Source.Value

	for event, matchers := range plugin.HooksConfig.Events {
		var pluginMatchers []PluginHookMatcher
		for _, m := range matchers {
			if len(m.Hooks) == 0 {
				continue
			}
			pluginMatchers = append(pluginMatchers, PluginHookMatcher{
				Matcher:    m.Matcher,
				Hooks:      m.Hooks,
				PluginRoot: plugin.Path,
				PluginName: plugin.Name,
				PluginID:   sourceID,
			})
		}
		result[event] = pluginMatchers
	}

	return result
}
