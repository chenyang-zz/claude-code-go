package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestGlobalSettingsStoreSaveEditorModePreservesExistingFields verifies editorMode updates keep unrelated settings content intact.
func TestGlobalSettingsStoreSaveEditorModePreservesExistingFields(t *testing.T) {
	tempDir := t.TempDir()
	settingsPath := filepath.Join(tempDir, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(settingsPath, []byte("{\n  \"model\": \"sonnet\"\n}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	store := &GlobalSettingsStore{Path: settingsPath}
	if err := store.SaveEditorMode(context.Background(), "vim"); err != nil {
		t.Fatalf("SaveEditorMode() error = %v", err)
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	got := string(data)
	want := "{\n  \"editorMode\": \"vim\",\n  \"model\": \"sonnet\"\n}\n"
	if got != want {
		t.Fatalf("settings content = %q, want %q", got, want)
	}
}

// TestGlobalSettingsStoreSaveEditorModeCreatesFile verifies the store can create a missing global settings file.
func TestGlobalSettingsStoreSaveEditorModeCreatesFile(t *testing.T) {
	tempDir := t.TempDir()
	settingsPath := filepath.Join(tempDir, ".claude", "settings.json")

	store := &GlobalSettingsStore{Path: settingsPath}
	if err := store.SaveEditorMode(context.Background(), "emacs"); err != nil {
		t.Fatalf("SaveEditorMode() error = %v", err)
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	got := string(data)
	want := "{\n  \"editorMode\": \"normal\"\n}\n"
	if got != want {
		t.Fatalf("settings content = %q, want %q", got, want)
	}
}

// TestGlobalSettingsStoreSaveThemePreservesExistingFields verifies theme updates keep unrelated settings content intact.
func TestGlobalSettingsStoreSaveThemePreservesExistingFields(t *testing.T) {
	tempDir := t.TempDir()
	settingsPath := filepath.Join(tempDir, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(settingsPath, []byte("{\n  \"model\": \"sonnet\"\n}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	store := &GlobalSettingsStore{Path: settingsPath}
	if err := store.SaveTheme(context.Background(), "light"); err != nil {
		t.Fatalf("SaveTheme() error = %v", err)
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	got := string(data)
	want := "{\n  \"model\": \"sonnet\",\n  \"theme\": \"light\"\n}\n"
	if got != want {
		t.Fatalf("settings content = %q, want %q", got, want)
	}
}

// TestGlobalSettingsStoreSaveThemeCreatesFile verifies the store can create a missing global settings file for theme writes.
func TestGlobalSettingsStoreSaveThemeCreatesFile(t *testing.T) {
	tempDir := t.TempDir()
	settingsPath := filepath.Join(tempDir, ".claude", "settings.json")

	store := &GlobalSettingsStore{Path: settingsPath}
	if err := store.SaveTheme(context.Background(), "dark"); err != nil {
		t.Fatalf("SaveTheme() error = %v", err)
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	got := string(data)
	want := "{\n  \"theme\": \"dark\"\n}\n"
	if got != want {
		t.Fatalf("settings content = %q, want %q", got, want)
	}
}
