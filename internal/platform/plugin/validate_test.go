package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateInstalledPlugin_Success(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(`{"name": "test-plugin"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := ValidateInstalledPlugin(dir); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateInstalledPlugin_NonExistentDir(t *testing.T) {
	err := ValidateInstalledPlugin("/nonexistent/path")
	if err == nil {
		t.Error("expected error for non-existent directory")
	}
}

func TestValidateInstalledPlugin_NotDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "not-a-dir")
	if err := os.WriteFile(srcFile, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := ValidateInstalledPlugin(srcFile)
	if err == nil {
		t.Error("expected error for non-directory path")
	}
}

func TestValidateInstalledPlugin_NoManifest(t *testing.T) {
	dir := t.TempDir()
	// No plugin.json file.

	err := ValidateInstalledPlugin(dir)
	if err == nil {
		t.Error("expected error for directory without manifest")
	}
}

func TestValidateInstalledPlugin_InvalidManifest(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(`{"name": ""}`), 0o644); err != nil {
		t.Fatal(err)
	}

	err := ValidateInstalledPlugin(dir)
	if err == nil {
		t.Error("expected error for invalid manifest")
	}
}

func TestValidateInstalledPlugin_MalformedJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(`{invalid json`), 0o644); err != nil {
		t.Fatal(err)
	}

	err := ValidateInstalledPlugin(dir)
	if err == nil {
		t.Error("expected error for malformed JSON")
	}
}
