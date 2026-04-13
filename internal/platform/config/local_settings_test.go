package config

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLocalSettingsStoreAddAdditionalDirectoryPreservesExistingFields verifies local settings updates keep unrelated content intact.
func TestLocalSettingsStoreAddAdditionalDirectoryPreservesExistingFields(t *testing.T) {
	projectDir := t.TempDir()
	settingsPath := filepath.Join(projectDir, ".claude", "settings.local.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	original := "{\n  \"theme\": \"light\",\n  \"permissions\": {\n    \"additionalDirectories\": [\"packages/app\"]\n  }\n}\n"
	if err := os.WriteFile(settingsPath, []byte(original), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	store := NewLocalSettingsStore(projectDir)
	if err := store.AddAdditionalDirectory(context.Background(), "/tmp/extra"); err != nil {
		t.Fatalf("AddAdditionalDirectory() error = %v", err)
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	got := string(data)
	if !strings.Contains(got, "\"theme\": \"light\"") {
		t.Fatalf("updated settings = %q, want unrelated fields preserved", got)
	}
	if !strings.Contains(got, "\"additionalDirectories\": [\n      \"packages/app\",\n      \"/tmp/extra\"\n    ]") {
		t.Fatalf("updated settings = %q, want appended additionalDirectories", got)
	}
}
