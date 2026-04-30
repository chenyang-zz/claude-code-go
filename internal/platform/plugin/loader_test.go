package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPluginLoader_LoadAll_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("CLAUDE_CODE_PLUGIN_CACHE_DIR", filepath.Join(tmpDir, "plugins"))

	store := NewInstalledPluginsStore()
	result, err := NewPluginLoader(store).LoadAll()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Enabled) != 0 {
		t.Errorf("expected 0 enabled plugins, got %d", len(result.Enabled))
	}
}

func TestPluginLoader_LoadAll_WithInstalledPlugin(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("CLAUDE_CODE_PLUGIN_CACHE_DIR", filepath.Join(tmpDir, "plugins"))

	// Go side LoadManifest expects plugin.json directly in plugin root.
	installPath := filepath.Join(tmpDir, "installed", "test-plugin")
	mustMkdirAll(t, installPath)
	writeFile(t, filepath.Join(installPath, "plugin.json"), `{"name": "test-plugin", "description": "A test plugin"}`)

	store := NewInstalledPluginsStore()
	err := store.AddPlugin("test-plugin", PluginInstallEntry{
		Scope:       "user",
		InstallPath: installPath,
		Version:     "1.0.0",
	})
	if err != nil {
		t.Fatalf("failed to add plugin: %v", err)
	}

	result, err := NewPluginLoader(store).LoadAll()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Enabled) != 1 {
		t.Fatalf("expected 1 enabled plugin, got %d", len(result.Enabled))
	}

	p := result.Enabled[0]
	if p.Name != "test-plugin" {
		t.Errorf("expected name 'test-plugin', got %q", p.Name)
	}
	if p.Path != installPath {
		t.Errorf("expected path %q, got %q", installPath, p.Path)
	}
	if !p.Enabled {
		t.Error("expected plugin to be enabled")
	}
}

func TestPluginLoader_LoadAll_MissingInstallPath(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("CLAUDE_CODE_PLUGIN_CACHE_DIR", filepath.Join(tmpDir, "plugins"))

	store := NewInstalledPluginsStore()
	err := store.AddPlugin("missing-plugin", PluginInstallEntry{
		Scope:       "user",
		InstallPath: filepath.Join(tmpDir, "nonexistent"),
		Version:     "1.0.0",
	})
	if err != nil {
		t.Fatalf("failed to add plugin: %v", err)
	}

	result, err := NewPluginLoader(store).LoadAll()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Enabled) != 0 {
		t.Errorf("expected 0 enabled plugins, got %d", len(result.Enabled))
	}
	if len(result.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(result.Errors))
	}
	if result.Errors[0].Type != "plugin-cache-miss" {
		t.Errorf("expected 'plugin-cache-miss', got %q", result.Errors[0].Type)
	}
}

func TestPluginLoader_LoadAll_InvalidManifest(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("CLAUDE_CODE_PLUGIN_CACHE_DIR", filepath.Join(tmpDir, "plugins"))

	installPath := filepath.Join(tmpDir, "installed", "bad-plugin")
	mustMkdirAll(t, installPath)
	writeFile(t, filepath.Join(installPath, "plugin.json"), `{"name": "", "description": "bad"}`)

	store := NewInstalledPluginsStore()
	err := store.AddPlugin("bad-plugin", PluginInstallEntry{
		Scope:       "user",
		InstallPath: installPath,
		Version:     "1.0.0",
	})
	if err != nil {
		t.Fatalf("failed to add plugin: %v", err)
	}

	result, err := NewPluginLoader(store).LoadAll()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Enabled) != 0 {
		t.Errorf("expected 0 enabled plugins, got %d", len(result.Enabled))
	}
	if len(result.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(result.Errors))
	}
	if result.Errors[0].Type != "manifest-load-error" {
		t.Errorf("expected 'manifest-load-error', got %q", result.Errors[0].Type)
	}
}

func TestPluginLoader_LoadAll_CapabilityDirDetection(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("CLAUDE_CODE_PLUGIN_CACHE_DIR", filepath.Join(tmpDir, "plugins"))

	installPath := filepath.Join(tmpDir, "installed", "full-plugin")
	mustMkdirAll(t, installPath)
	writeFile(t, filepath.Join(installPath, "plugin.json"), `{"name": "full-plugin", "description": "Has capabilities"}`)

	// Create capability directories.
	mustMkdirAll(t, filepath.Join(installPath, "commands"))
	mustMkdirAll(t, filepath.Join(installPath, "skills"))
	mustMkdirAll(t, filepath.Join(installPath, "agents"))
	mustMkdirAll(t, filepath.Join(installPath, "output-styles"))

	store := NewInstalledPluginsStore()
	err := store.AddPlugin("full-plugin", PluginInstallEntry{
		Scope:       "user",
		InstallPath: installPath,
		Version:     "1.0.0",
	})
	if err != nil {
		t.Fatalf("failed to add plugin: %v", err)
	}

	result, err := NewPluginLoader(store).LoadAll()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Enabled) != 1 {
		t.Fatalf("expected 1 enabled plugin, got %d", len(result.Enabled))
	}

	p := result.Enabled[0]
	if p.CommandsPath == "" {
		t.Error("expected CommandsPath to be set")
	}
	if p.SkillsPath == "" {
		t.Error("expected SkillsPath to be set")
	}
	if p.AgentsPath == "" {
		t.Error("expected AgentsPath to be set")
	}
	if p.OutputStylesPath == "" {
		t.Error("expected OutputStylesPath to be set")
	}
}

func TestPluginLoader_LoadAll_WithHooksConfig(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("CLAUDE_CODE_PLUGIN_CACHE_DIR", filepath.Join(tmpDir, "plugins"))

	installPath := filepath.Join(tmpDir, "installed", "hook-plugin")
	hooksDir := filepath.Join(installPath, "hooks")
	mustMkdirAll(t, installPath)
	mustMkdirAll(t, hooksDir)
	writeFile(t, filepath.Join(installPath, "plugin.json"), `{"name": "hook-plugin", "description": "Has hooks"}`)
	writeFile(t, filepath.Join(hooksDir, "hooks.json"), `{
		"events": {
			"Stop": [{"matcher": "", "hooks": [{"type": "command", "command": "echo done"}]}]
		}
	}`)

	store := NewInstalledPluginsStore()
	err := store.AddPlugin("hook-plugin", PluginInstallEntry{
		Scope:       "user",
		InstallPath: installPath,
		Version:     "1.0.0",
	})
	if err != nil {
		t.Fatalf("failed to add plugin: %v", err)
	}

	result, err := NewPluginLoader(store).LoadAll()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Enabled) != 1 {
		t.Fatalf("expected 1 enabled plugin, got %d", len(result.Enabled))
	}

	p := result.Enabled[0]
	if p.HooksConfig == nil {
		t.Fatal("expected HooksConfig to be set")
	}
	if len(p.HooksConfig.Events) != 1 {
		t.Errorf("expected 1 event, got %d", len(p.HooksConfig.Events))
	}
}

func TestPluginLoader_LoadAll_BrokenHooksJSON(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("CLAUDE_CODE_PLUGIN_CACHE_DIR", filepath.Join(tmpDir, "plugins"))

	installPath := filepath.Join(tmpDir, "installed", "broken-hook-plugin")
	hooksDir := filepath.Join(installPath, "hooks")
	mustMkdirAll(t, installPath)
	mustMkdirAll(t, hooksDir)
	writeFile(t, filepath.Join(installPath, "plugin.json"), `{"name": "broken-hook-plugin", "description": "Broken"}`)
	writeFile(t, filepath.Join(hooksDir, "hooks.json"), `{invalid json}`)

	store := NewInstalledPluginsStore()
	err := store.AddPlugin("broken-hook-plugin", PluginInstallEntry{
		Scope:       "user",
		InstallPath: installPath,
		Version:     "1.0.0",
	})
	if err != nil {
		t.Fatalf("failed to add plugin: %v", err)
	}

	result, err := NewPluginLoader(store).LoadAll()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	// Plugin should still load despite broken hooks.json.
	if len(result.Enabled) != 1 {
		t.Errorf("expected 1 enabled plugin, got %d", len(result.Enabled))
	}
	if len(result.Errors) != 1 {
		t.Errorf("expected 1 error, got %d", len(result.Errors))
	}
	if result.Errors[0].Type != "hook-load-failed" {
		t.Errorf("expected 'hook-load-failed', got %q", result.Errors[0].Type)
	}
}

func TestPluginLoader_LoadAll_MultiplePlugins(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("CLAUDE_CODE_PLUGIN_CACHE_DIR", filepath.Join(tmpDir, "plugins"))

	store := NewInstalledPluginsStore()

	for i, name := range []string{"plugin-a", "plugin-b", "plugin-c"} {
		installPath := filepath.Join(tmpDir, "installed", name)
		mustMkdirAll(t, installPath)
		writeFile(t, filepath.Join(installPath, "plugin.json"),
			`{"name": "`+name+`", "description": "Plugin `+string(rune('A'+i))+`"}`)

		err := store.AddPlugin(name, PluginInstallEntry{
			Scope:       "user",
			InstallPath: installPath,
			Version:     "1.0.0",
		})
		if err != nil {
			t.Fatalf("failed to add plugin %s: %v", name, err)
		}
	}

	result, err := NewPluginLoader(store).LoadAll()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Enabled) != 3 {
		t.Errorf("expected 3 enabled plugins, got %d", len(result.Enabled))
	}
}

func TestPluginLoader_LoadAll_SourceTypeInference(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("CLAUDE_CODE_PLUGIN_CACHE_DIR", filepath.Join(tmpDir, "plugins"))

	store := NewInstalledPluginsStore()

	// Git source plugin.
	gitPath := filepath.Join(tmpDir, "installed", "git-plugin")
	mustMkdirAll(t, gitPath)
	writeFile(t, filepath.Join(gitPath, "plugin.json"), `{"name": "git-plugin"}`)
	store.AddPlugin("git-plugin", PluginInstallEntry{
		Scope:        "user",
		InstallPath:  gitPath,
		GitCommitSha: "abc123",
	})

	// Path source plugin.
	pathPluginDir := filepath.Join(tmpDir, "installed", "path-plugin")
	mustMkdirAll(t, pathPluginDir)
	writeFile(t, filepath.Join(pathPluginDir, "plugin.json"), `{"name": "path-plugin"}`)
	store.AddPlugin("path-plugin", PluginInstallEntry{
		Scope:       "user",
		InstallPath: pathPluginDir,
	})

	result, _ := NewPluginLoader(store).LoadAll()
	if len(result.Enabled) != 2 {
		t.Fatalf("expected 2 plugins, got %d", len(result.Enabled))
	}

	for _, p := range result.Enabled {
		switch p.Name {
		case "git-plugin":
			if p.Source.Type != SourceTypeGit {
				t.Errorf("expected git source type, got %s", p.Source.Type)
			}
		case "path-plugin":
			if p.Source.Type != SourceTypePath {
				t.Errorf("expected path source type, got %s", p.Source.Type)
			}
		}
	}
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("failed to create directory %s: %v", path, err)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write file %s: %v", path, err)
	}
}
