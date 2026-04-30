package plugin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSanitizePluginID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"my-plugin", "my-plugin"},
		{"my_plugin", "my_plugin"},
		{"my.plugin", "my-plugin"},
		{"my/plugin", "my-plugin"},
		{"my:plugin", "my-plugin"},
		{"my plugin", "my-plugin"},
		{"my@plugin", "my-plugin"},
		{"name@marketplace", "name-marketplace"},
		{"owner/repo", "owner-repo"},
		{"a.b.c.d", "a-b-c-d"},
		{"", ""},
	}

	for _, tt := range tests {
		got := SanitizePluginID(tt.input)
		if got != tt.expected {
			t.Errorf("SanitizePluginID(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestPluginDataDirPath(t *testing.T) {
	// Override plugins dir for testing.
	origDir := GetPluginsDir()
	t.Cleanup(func() {
		// Note: GetPluginsDir reads env each time, so no restoration needed.
		_ = origDir
	})

	t.Setenv("CLAUDE_CODE_PLUGIN_CACHE_DIR", "/tmp/test-plugins")

	path := PluginDataDirPath("my-plugin")
	expected := filepath.Join("/tmp/test-plugins", "data", "my-plugin")
	if path != expected {
		t.Errorf("PluginDataDirPath() = %q, want %q", path, expected)
	}
}

func TestGetPluginDataDir_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("CLAUDE_CODE_PLUGIN_CACHE_DIR", tmpDir)

	dir, err := GetPluginDataDir("test-plugin")
	if err != nil {
		t.Fatalf("GetPluginDataDir failed: %v", err)
	}

	expected := filepath.Join(tmpDir, "data", "test-plugin")
	if dir != expected {
		t.Errorf("GetPluginDataDir() = %q, want %q", dir, expected)
	}

	// Verify the directory was created.
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("expected directory to exist: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected a directory, got a file")
	}
}

func TestGetPluginDataDir_SanitizesID(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("CLAUDE_CODE_PLUGIN_CACHE_DIR", tmpDir)

	dir, err := GetPluginDataDir("my/plugin")
	if err != nil {
		t.Fatalf("GetPluginDataDir failed: %v", err)
	}

	expected := filepath.Join(tmpDir, "data", "my-plugin")
	if dir != expected {
		t.Errorf("GetPluginDataDir() = %q, want %q", dir, expected)
	}

	// Verify the sanitized directory was created.
	if _, err := os.Stat(expected); err != nil {
		t.Fatalf("expected sanitized directory to exist: %v", err)
	}
}

func TestGetPluginDataDir_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("CLAUDE_CODE_PLUGIN_CACHE_DIR", tmpDir)

	// First call creates the directory.
	_, err := GetPluginDataDir("existing-plugin")
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}

	// Second call should succeed without error.
	_, err = GetPluginDataDir("existing-plugin")
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}
}

func TestPluginDataDirPath_UsesHomeDir(t *testing.T) {
	// When CLAUDE_CODE_PLUGIN_CACHE_DIR is not set, it should fall back to ~/.claude/plugins.
	os.Unsetenv("CLAUDE_CODE_PLUGIN_CACHE_DIR")

	path := PluginDataDirPath("my-plugin")
	if !strings.Contains(path, ".claude") {
		t.Errorf("expected path to contain '.claude', got %q", path)
	}
	if !strings.Contains(path, "data") {
		t.Errorf("expected path to contain 'data', got %q", path)
	}
	if !strings.Contains(path, "my-plugin") {
		t.Errorf("expected path to contain 'my-plugin', got %q", path)
	}
}
