package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUninstallPlugin_UserScope(t *testing.T) {
	tmpDir := t.TempDir()
	pluginsDir := filepath.Join(tmpDir, "plugins")
	pluginDir := filepath.Join(pluginsDir, "test-plugin")

	// Create the installed plugin directory.
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.json"), []byte(`{"name": "test-plugin"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	// Override plugins dir via environment variable for testing.
	t.Setenv("CLAUDE_CODE_PLUGIN_CACHE_DIR", pluginsDir)

	if err := UninstallPlugin("test-plugin", "user", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the directory was removed.
	if _, err := os.Stat(pluginDir); !os.IsNotExist(err) {
		t.Error("expected plugin directory to be removed")
	}
}

func TestUninstallPlugin_EmptyName(t *testing.T) {
	err := UninstallPlugin("", "user", "")
	if err == nil {
		t.Error("expected error for empty plugin name")
	}
}

func TestUninstallPlugin_UnknownScope(t *testing.T) {
	err := UninstallPlugin("test-plugin", "workspace", "")
	if err == nil {
		t.Error("expected error for unknown scope")
	}
}

func TestUninstallPlugin_ProjectScope_CleansUp(t *testing.T) {
	tmpDir := t.TempDir()
	pluginsDir := filepath.Join(tmpDir, ".claude", "plugins")
	pluginDir := filepath.Join(pluginsDir, "test-plugin")

	// Create the installed plugin directory.
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.json"), []byte(`{"name": "test-plugin"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := UninstallPlugin("test-plugin", "project", tmpDir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the directory was removed.
	if _, err := os.Stat(pluginDir); !os.IsNotExist(err) {
		t.Error("expected plugin directory to be removed")
	}
}
