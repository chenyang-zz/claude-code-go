package config

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestProjectSettingsStoreAddAdditionalDirectoryPreservesExistingFields verifies project additionalDirectories updates keep unrelated settings content intact.
func TestProjectSettingsStoreAddAdditionalDirectoryPreservesExistingFields(t *testing.T) {
	tempDir := t.TempDir()
	settingsPath := filepath.Join(tempDir, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	original := "{\n  \"theme\": \"light\",\n  \"permissions\": {\n    \"allow\": [\"Bash(ls)\"],\n    \"additionalDirectories\": [\"packages/app\"]\n  }\n}\n"
	if err := os.WriteFile(settingsPath, []byte(original), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	store := &ProjectSettingsStore{Path: settingsPath}
	if err := store.AddAdditionalDirectory(context.Background(), "/tmp/extra"); err != nil {
		t.Fatalf("AddAdditionalDirectory() error = %v", err)
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	got := string(data)
	if !strings.Contains(got, "\"theme\": \"light\"") {
		t.Fatalf("updated settings = %q, want preserved theme", got)
	}
	if !strings.Contains(got, "\"allow\": [\n      \"Bash(ls)\"\n    ]") {
		t.Fatalf("updated settings = %q, want preserved allow rules", got)
	}
	if !strings.Contains(got, "\"additionalDirectories\": [\n      \"packages/app\",\n      \"/tmp/extra\"\n    ]") {
		t.Fatalf("updated settings = %q, want appended additionalDirectories", got)
	}
}

// TestProjectSettingsStoreAddAdditionalDirectoryDeduplicates verifies the store does not append the same directory twice.
func TestProjectSettingsStoreAddAdditionalDirectoryDeduplicates(t *testing.T) {
	tempDir := t.TempDir()
	settingsPath := filepath.Join(tempDir, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(settingsPath, []byte("{\"permissions\":{\"additionalDirectories\":[\"/tmp/extra\"]}}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	store := &ProjectSettingsStore{Path: settingsPath}
	if err := store.AddAdditionalDirectory(context.Background(), "/tmp/extra"); err != nil {
		t.Fatalf("AddAdditionalDirectory() error = %v", err)
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if strings.Count(string(data), "/tmp/extra") != 1 {
		t.Fatalf("updated settings = %q, want one /tmp/extra entry", string(data))
	}
}

// TestProjectSettingsStoreAddAdditionalDirectoryCreatesFile verifies the store can create a missing project settings file.
func TestProjectSettingsStoreAddAdditionalDirectoryCreatesFile(t *testing.T) {
	tempDir := t.TempDir()
	settingsPath := filepath.Join(tempDir, ".claude", "settings.json")

	store := &ProjectSettingsStore{Path: settingsPath}
	if err := store.AddAdditionalDirectory(context.Background(), "/tmp/extra"); err != nil {
		t.Fatalf("AddAdditionalDirectory() error = %v", err)
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if got := string(data); !strings.Contains(got, "\"additionalDirectories\": [\n      \"/tmp/extra\"\n    ]") {
		t.Fatalf("created settings = %q, want persisted /tmp/extra", got)
	}
}
