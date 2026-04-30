package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestComputeInstallPath_UserScope(t *testing.T) {
	opts := InstallOptions{Scope: "user"}
	path, err := computeInstallPath("test-plugin", opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path == "" {
		t.Error("expected non-empty path")
	}
}

func TestComputeInstallPath_ProjectScope(t *testing.T) {
	tmpDir := t.TempDir()
	opts := InstallOptions{Scope: "project", ProjectDir: tmpDir}
	path, err := computeInstallPath("test-plugin", opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expectedDir := filepath.Join(tmpDir, ".claude", "plugins", "test-plugin")
	if path != expectedDir {
		t.Errorf("expected %s, got %s", expectedDir, path)
	}
}

func TestComputeInstallPath_ProjectScopeMissingDir(t *testing.T) {
	opts := InstallOptions{Scope: "project"}
	_, err := computeInstallPath("test-plugin", opts)
	if err == nil {
		t.Error("expected error for missing ProjectDir")
	}
}

func TestComputeInstallPath_EmptyName(t *testing.T) {
	opts := InstallOptions{Scope: "user"}
	_, err := computeInstallPath("", opts)
	if err == nil {
		t.Error("expected error for empty plugin name")
	}
}

func TestComputeInstallPath_UnknownScope(t *testing.T) {
	opts := InstallOptions{Scope: "workspace"}
	_, err := computeInstallPath("test-plugin", opts)
	if err == nil {
		t.Error("expected error for unknown scope")
	}
}

func TestComputeInstallPath_ExistingDir(t *testing.T) {
	tmpDir := t.TempDir()
	existingDir := filepath.Join(tmpDir, "plugins", "existing-plugin")
	if err := os.MkdirAll(existingDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Write a file so the directory is non-empty.
	if err := os.WriteFile(filepath.Join(existingDir, "foo.txt"), []byte("bar"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Override plugins dir via environment variable for testing.
	t.Setenv("CLAUDE_CODE_PLUGIN_CACHE_DIR", filepath.Join(tmpDir, "plugins"))

	opts := InstallOptions{Scope: "user"}
	_, err := computeInstallPath("existing-plugin", opts)
	if err == nil {
		t.Error("expected error for existing non-empty directory")
	}
}

func TestAcquireFromPath_SimpleCopy(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	// Create a source file.
	srcFile := filepath.Join(src, "plugin.json")
	if err := os.WriteFile(srcFile, []byte(`{"name": "test"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	targetDir := filepath.Join(dst, "plugin")
	if err := acquireFromPath(src, targetDir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the file was copied.
	dstFile := filepath.Join(targetDir, "plugin.json")
	if _, err := os.Stat(dstFile); os.IsNotExist(err) {
		t.Error("expected plugin.json to be copied")
	}
}

func TestAcquireFromPath_RecursiveCopy(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	// Create a nested structure.
	subDir := filepath.Join(src, "subdir")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "top.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "nested.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	targetDir := filepath.Join(dst, "plugin")
	if err := acquireFromPath(src, targetDir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify both files exist.
	for _, p := range []string{"top.json", "subdir/nested.json"} {
		if _, err := os.Stat(filepath.Join(targetDir, p)); os.IsNotExist(err) {
			t.Errorf("expected %s to be copied", p)
		}
	}
}

func TestAcquireFromPath_SkipsSymlink(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	// Create a regular file.
	srcFile := filepath.Join(src, "real.json")
	if err := os.WriteFile(srcFile, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a symlink.
	srcLink := filepath.Join(src, "link.json")
	if err := os.Symlink(srcFile, srcLink); err != nil {
		t.Fatal(err)
	}

	targetDir := filepath.Join(dst, "plugin")
	if err := acquireFromPath(src, targetDir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Real file should exist.
	if _, err := os.Stat(filepath.Join(targetDir, "real.json")); os.IsNotExist(err) {
		t.Error("expected real.json to be copied")
	}

	// Symlink should NOT exist.
	if _, err := os.Lstat(filepath.Join(targetDir, "link.json")); !os.IsNotExist(err) {
		t.Error("expected link.json (symlink) to be skipped")
	}
}

func TestCopyFile(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "src.txt")
	if err := os.WriteFile(src, []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(tmpDir, "dst.txt")
	if err := copyFile(src, dst); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello world" {
		t.Errorf("expected 'hello world', got '%s'", string(data))
	}
}

func TestInstallPlugin_PathSource(t *testing.T) {
	src := t.TempDir()
	// Create a minimal plugin directory.
	if err := os.WriteFile(filepath.Join(src, "plugin.json"), []byte(`{"name": "my-plugin"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := InstallPlugin(PluginSource{Type: SourceTypePath, Value: src}, InstallOptions{Scope: "user"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Plugin.Name != "my-plugin" {
		t.Errorf("expected plugin name 'my-plugin', got '%s'", result.Plugin.Name)
	}
	if result.InstallPath == "" {
		t.Error("expected non-empty install path")
	}

	// Clean up installed plugin.
	defer os.RemoveAll(result.InstallPath)
}

func TestInstallPlugin_UnsupportedSource(t *testing.T) {
	_, err := InstallPlugin(PluginSource{Type: SourceTypeNPM, Value: "some-package"}, InstallOptions{Scope: "user"})
	if err == nil {
		t.Error("expected error for unsupported source type")
	}
}

func TestInstallPlugin_PathSourceNonExistent(t *testing.T) {
	_, err := InstallPlugin(PluginSource{Type: SourceTypePath, Value: "/nonexistent/path"}, InstallOptions{Scope: "user"})
	if err == nil {
		t.Error("expected error for non-existent path")
	}
}

func TestInstallPlugin_PathSourceNotDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "not-a-dir")
	if err := os.WriteFile(srcFile, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := InstallPlugin(PluginSource{Type: SourceTypePath, Value: srcFile}, InstallOptions{Scope: "user"})
	if err == nil {
		t.Error("expected error for non-directory source")
	}
}

func TestInstallPlugin_PathSourceNoManifest(t *testing.T) {
	src := t.TempDir()
	// No plugin.json in the source directory.

	_, err := InstallPlugin(PluginSource{Type: SourceTypePath, Value: src}, InstallOptions{Scope: "user"})
	if err == nil {
		t.Error("expected error for source without manifest")
	}
}
