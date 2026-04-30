package plugin

import (
	"path/filepath"
	"testing"
)

func TestRefreshActivePlugins_EmptyStore(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("CLAUDE_CODE_PLUGIN_CACHE_DIR", filepath.Join(tmpDir, "plugins"))

	store := NewInstalledPluginsStore()
	result, err := NewPluginLoader(store).RefreshActivePlugins()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.EnabledCount != 0 {
		t.Errorf("expected 0 enabled, got %d", result.EnabledCount)
	}
	if result.DisabledCount != 0 {
		t.Errorf("expected 0 disabled, got %d", result.DisabledCount)
	}
}

func TestRefreshActivePlugins_WithPlugins(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("CLAUDE_CODE_PLUGIN_CACHE_DIR", filepath.Join(tmpDir, "plugins"))

	installPath := filepath.Join(tmpDir, "installed", "refresh-test")
	mustMkdirAll(t, installPath)
	writeFile(t, filepath.Join(installPath, "plugin.json"), `{"name": "refresh-test", "description": "Test refresh"}`)

	store := NewInstalledPluginsStore()
	err := store.AddPlugin("refresh-test", PluginInstallEntry{
		Scope:       "user",
		InstallPath: installPath,
		Version:     "1.0.0",
	})
	if err != nil {
		t.Fatalf("failed to add plugin: %v", err)
	}

	result, err := NewPluginLoader(store).RefreshActivePlugins()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.EnabledCount != 1 {
		t.Errorf("expected 1 enabled, got %d", result.EnabledCount)
	}
}

func TestRefreshActivePlugins_WithCapabilities(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("CLAUDE_CODE_PLUGIN_CACHE_DIR", filepath.Join(tmpDir, "plugins"))

	installPath := filepath.Join(tmpDir, "installed", "full-plugin")
	mustMkdirAll(t, installPath)
	writeFile(t, filepath.Join(installPath, "plugin.json"), `{"name": "full-plugin", "description": "Has capabilities"}`)

	// Create commands directory with a command.
	cmdPath := filepath.Join(installPath, "commands")
	mustMkdirAll(t, cmdPath)
	writeFile(t, filepath.Join(cmdPath, "hello.md"), `---
name: hello
description: Say hello
---
# Hello`)

	// Create output-styles directory with a style.
	stylePath := filepath.Join(installPath, "output-styles")
	mustMkdirAll(t, stylePath)
	writeFile(t, filepath.Join(stylePath, "compact.md"), `---
name: compact
---
Use compact formatting.`)

	// Create hooks directory with hooks.json.
	hooksPath := filepath.Join(installPath, "hooks")
	mustMkdirAll(t, hooksPath)
	writeFile(t, filepath.Join(hooksPath, "hooks.json"), `{
		"events": {
			"Stop": [{"matcher": "", "hooks": [{"type": "command", "command": "echo done"}]}]
		}
	}`)

	store := NewInstalledPluginsStore()
	err := store.AddPlugin("full-plugin", PluginInstallEntry{
		Scope:       "user",
		InstallPath: installPath,
		Version:     "1.0.0",
	})
	if err != nil {
		t.Fatalf("failed to add plugin: %v", err)
	}

	result, err := NewPluginLoader(store).RefreshActivePlugins()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.EnabledCount != 1 {
		t.Errorf("expected 1 enabled, got %d", result.EnabledCount)
	}
	if result.CommandCount != 1 {
		t.Errorf("expected 1 command, got %d", result.CommandCount)
	}
	if result.HookCount != 1 {
		t.Errorf("expected 1 hook matcher, got %d", result.HookCount)
	}
}

func TestRefreshActivePlugins_ErrorCount(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("CLAUDE_CODE_PLUGIN_CACHE_DIR", filepath.Join(tmpDir, "plugins"))

	store := NewInstalledPluginsStore()
	// Add a plugin with a non-existent install path.
	err := store.AddPlugin("missing", PluginInstallEntry{
		Scope:       "user",
		InstallPath: filepath.Join(tmpDir, "nonexistent"),
		Version:     "1.0.0",
	})
	if err != nil {
		t.Fatalf("failed to add plugin: %v", err)
	}

	result, err := NewPluginLoader(store).RefreshActivePlugins()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.ErrorCount == 0 {
		t.Error("expected error count > 0 for missing plugin")
	}
}
