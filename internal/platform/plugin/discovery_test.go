package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetPluginsDirDefault(t *testing.T) {
	// Ensure no override env is set.
	os.Unsetenv("CLAUDE_CODE_PLUGIN_CACHE_DIR")

	dir := GetPluginsDir()
	if dir == "" {
		t.Error("expected non-empty plugins dir")
	}

	home, err := os.UserHomeDir()
	if err == nil {
		expected := filepath.Join(home, ".claude", defaultPluginsDirName)
		if dir != expected {
			t.Errorf("expected %q, got %q", expected, dir)
		}
	}
}

func TestGetPluginsDirEnvOverride(t *testing.T) {
	customDir := t.TempDir()
	t.Setenv("CLAUDE_CODE_PLUGIN_CACHE_DIR", customDir)

	dir := GetPluginsDir()
	if dir != customDir {
		t.Errorf("expected env override %q, got %q", customDir, dir)
	}
}

func TestDiscoverPluginDirsEmpty(t *testing.T) {
	dir := t.TempDir()

	dirs, err := DiscoverPluginDirs(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(dirs) != 0 {
		t.Errorf("expected 0 dirs in empty directory, got %d", len(dirs))
	}
}

func TestDiscoverPluginDirsNonExistent(t *testing.T) {
	dirs, err := DiscoverPluginDirs("/nonexistent/path/for/test")
	if err != nil {
		t.Fatalf("unexpected error for non-existent dir: %v", err)
	}
	if dirs != nil {
		t.Errorf("expected nil for non-existent dir, got %v", dirs)
	}
}

func TestDiscoverPluginDirsWithManifest(t *testing.T) {
	dir := t.TempDir()

	// Create a plugin dir with plugin.json.
	pluginDir := filepath.Join(dir, "my-plugin")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatalf("failed to create plugin dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.json"), []byte(`{"name":"my-plugin"}`), 0o644); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	// Create a dir without manifest (should be skipped).
	noPluginDir := filepath.Join(dir, "not-a-plugin")
	if err := os.MkdirAll(noPluginDir, 0o755); err != nil {
		t.Fatalf("failed to create non-plugin dir: %v", err)
	}

	// Create a file (should be skipped by IsDir check).
	if err := os.WriteFile(filepath.Join(dir, "random-file"), []byte("data"), 0o644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	dirs, err := DiscoverPluginDirs(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(dirs) != 1 {
		t.Errorf("expected 1 plugin dir, got %d", len(dirs))
	}
	if len(dirs) > 0 && dirs[0] != pluginDir {
		t.Errorf("expected %q, got %q", pluginDir, dirs[0])
	}
}

func TestDiscoverPluginDirsWithPackageJSON(t *testing.T) {
	dir := t.TempDir()

	pluginDir := filepath.Join(dir, "pkg-json-plugin")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatalf("failed to create plugin dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "package.json"), []byte(`{"name":"pkg-json-plugin"}`), 0o644); err != nil {
		t.Fatalf("failed to write package.json: %v", err)
	}

	dirs, err := DiscoverPluginDirs(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(dirs) != 1 {
		t.Errorf("expected 1 plugin dir with package.json, got %d", len(dirs))
	}
}

func TestDiscoverPluginDirsPluginJSONPrecedence(t *testing.T) {
	// Both plugin.json and package.json exist — dir should still only appear once.
	dir := t.TempDir()

	pluginDir := filepath.Join(dir, "my-plugin")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatalf("failed to create plugin dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.json"), []byte(`{"name":"my-plugin"}`), 0o644); err != nil {
		t.Fatalf("failed to write plugin.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "package.json"), []byte(`{"name":"my-plugin"}`), 0o644); err != nil {
		t.Fatalf("failed to write package.json: %v", err)
	}

	dirs, err := DiscoverPluginDirs(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(dirs) != 1 {
		t.Errorf("expected 1 dir (no duplicates), got %d", len(dirs))
	}
}

func TestHasManifestFilePositive(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatalf("failed to write plugin.json: %v", err)
	}
	if !hasManifestFile(dir) {
		t.Error("expected hasManifestFile to return true")
	}
}

func TestHasManifestFileNegative(t *testing.T) {
	dir := t.TempDir()
	if hasManifestFile(dir) {
		t.Error("expected hasManifestFile to return false for empty dir")
	}
}

func TestHasManifestFileIgnoresDirectories(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "plugin.json")
	if err := os.Mkdir(manifestPath, 0o755); err != nil {
		t.Fatalf("failed to create dir named plugin.json: %v", err)
	}
	if hasManifestFile(dir) {
		t.Error("expected hasManifestFile to return false when manifest name is a directory")
	}
}

func TestDiscoverAllPluginDirs(t *testing.T) {
	// Create a project-level .claude/plugins/ directory.
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)

	tmpRoot := t.TempDir()
	os.Chdir(tmpRoot)

	projectPluginDir := filepath.Join(tmpRoot, ".claude", defaultPluginsDirName, "project-plugin")
	if err := os.MkdirAll(projectPluginDir, 0o755); err != nil {
		t.Fatalf("failed to create project plugin dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectPluginDir, "plugin.json"), []byte(`{"name":"project-plugin"}`), 0o644); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	dirs := DiscoverAllPluginDirs()
	// On macOS /tmp may be a symlink to /private/tmp, so compare by
	// resolving symlinks.
	resolvedProjectDir, _ := filepath.EvalSymlinks(projectPluginDir)
	found := false
	for _, d := range dirs {
		if d == projectPluginDir || d == resolvedProjectDir {
			found = true
			break
		}
		// Also try resolving the discovered path.
		if resolved, err := filepath.EvalSymlinks(d); err == nil && resolved == resolvedProjectDir {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected to discover project plugin at %q, got %v", projectPluginDir, dirs)
	}
}
