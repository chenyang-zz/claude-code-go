package plugin

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestLoadHooksConfig_FileNotFound(t *testing.T) {
	config, err := LoadHooksConfig(t.TempDir())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if config != nil {
		t.Error("expected nil config for missing file")
	}
}

func TestLoadHooksConfig_Valid(t *testing.T) {
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks")
	mustMkdirAll(t, hooksDir)
	writeFile(t, filepath.Join(hooksDir, "hooks.json"), `{
		"events": {
			"Stop": [
				{
					"matcher": "",
					"hooks": [{"type": "command", "command": "echo done"}]
				}
			],
			"PreToolUse": [
				{
					"matcher": "Bash.*",
					"hooks": [{"type": "command", "command": "echo before", "timeout": 5000}]
				}
			]
		}
	}`)

	config, err := LoadHooksConfig(tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if config == nil {
		t.Fatal("expected non-nil config")
	}
	if len(config.Events) != 2 {
		t.Errorf("expected 2 events, got %d", len(config.Events))
	}
	if len(config.Events["Stop"]) != 1 {
		t.Errorf("expected 1 Stop matcher, got %d", len(config.Events["Stop"]))
	}
	if len(config.Events["PreToolUse"]) != 1 {
		t.Errorf("expected 1 PreToolUse matcher, got %d", len(config.Events["PreToolUse"]))
	}
}

func TestLoadHooksConfig_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks")
	mustMkdirAll(t, hooksDir)
	writeFile(t, filepath.Join(hooksDir, "hooks.json"), `{invalid}`)

	_, err := LoadHooksConfig(tmpDir)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestExtractHooks_NilConfig(t *testing.T) {
	plugin := &LoadedPlugin{
		Name:        "test-plugin",
		Path:        "/tmp/test",
		HooksConfig: nil,
	}

	result := ExtractHooks(plugin)
	if len(result) != len(HookEventNames) {
		t.Errorf("expected %d events, got %d", len(HookEventNames), len(result))
	}
	// All events should have nil/empty slices.
	for _, event := range HookEventNames {
		if _, ok := result[event]; !ok {
			t.Errorf("expected event %q to be present", event)
		}
	}
}

func TestExtractHooks_EmptyConfig(t *testing.T) {
	plugin := &LoadedPlugin{
		Name: "test-plugin",
		Path: "/tmp/test",
		HooksConfig: &HooksConfig{
			Events: map[string][]HookMatcherEntry{},
		},
	}

	result := ExtractHooks(plugin)
	if len(result) != len(HookEventNames) {
		t.Errorf("expected %d events, got %d", len(HookEventNames), len(result))
	}
}

func TestExtractHooks_WithHooks(t *testing.T) {
	plugin := &LoadedPlugin{
		Name: "test-plugin",
		Path: "/tmp/test-plugin",
		Source: PluginSource{
			Type:  SourceTypePath,
			Value: "/tmp/test-plugin",
		},
		HooksConfig: &HooksConfig{
			Events: map[string][]HookMatcherEntry{
				"Stop": {
					{
						Matcher: "",
						Hooks:   []HookCommand{{Type: "command", Command: "echo done"}},
					},
				},
				"PreToolUse": {
					{
						Matcher: "Bash.*",
						Hooks:   []HookCommand{{Type: "command", Command: "echo before", Timeout: 5000}},
					},
				},
			},
		},
	}

	result := ExtractHooks(plugin)

	stopHooks := result["Stop"]
	if len(stopHooks) != 1 {
		t.Fatalf("expected 1 Stop hook, got %d", len(stopHooks))
	}
	if stopHooks[0].PluginName != "test-plugin" {
		t.Errorf("expected PluginName 'test-plugin', got %q", stopHooks[0].PluginName)
	}
	if stopHooks[0].PluginRoot != "/tmp/test-plugin" {
		t.Errorf("expected PluginRoot '/tmp/test-plugin', got %q", stopHooks[0].PluginRoot)
	}
	if stopHooks[0].PluginID != "test-plugin@/tmp/test-plugin" {
		t.Errorf("expected PluginID 'test-plugin@/tmp/test-plugin', got %q", stopHooks[0].PluginID)
	}

	preHooks := result["PreToolUse"]
	if len(preHooks) != 1 {
		t.Fatalf("expected 1 PreToolUse hook, got %d", len(preHooks))
	}
	if preHooks[0].Matcher != "Bash.*" {
		t.Errorf("expected matcher 'Bash.*', got %q", preHooks[0].Matcher)
	}
	if preHooks[0].Hooks[0].Timeout != 5000 {
		t.Errorf("expected timeout 5000, got %d", preHooks[0].Hooks[0].Timeout)
	}
}

func TestExtractHooks_SkipsEmptyHooks(t *testing.T) {
	plugin := &LoadedPlugin{
		Name: "test-plugin",
		Path: "/tmp/test-plugin",
		Source: PluginSource{
			Type:  SourceTypePath,
			Value: "/tmp/test-plugin",
		},
		HooksConfig: &HooksConfig{
			Events: map[string][]HookMatcherEntry{
				"Stop": {
					{
						Matcher: "test",
						Hooks:   []HookCommand{}, // Empty hooks — should be skipped.
					},
					{
						Matcher: "",
						Hooks:   []HookCommand{{Type: "command", Command: "echo real"}},
					},
				},
			},
		},
	}

	result := ExtractHooks(plugin)
	stopHooks := result["Stop"]
	if len(stopHooks) != 1 {
		t.Errorf("expected 1 Stop hook (empty skipped), got %d", len(stopHooks))
	}
}

func TestHookEventNames(t *testing.T) {
	// Verify all 27 hook event names are valid JSON and non-empty.
	data, err := json.Marshal(HookEventNames)
	if err != nil {
		t.Fatalf("expected valid JSON, got error: %v", err)
	}
	var names []string
	if err := json.Unmarshal(data, &names); err != nil {
		t.Fatalf("expected valid JSON, got error: %v", err)
	}
	if len(names) != 27 {
		t.Errorf("expected 27 hook events, got %d", len(names))
	}
	for i, name := range names {
		if name == "" {
			t.Errorf("event name at index %d is empty", i)
		}
	}
}
