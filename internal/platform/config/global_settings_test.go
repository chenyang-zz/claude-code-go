package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestGlobalSettingsStoreSaveModelPreservesExistingFields verifies model updates keep unrelated settings content intact.
func TestGlobalSettingsStoreSaveModelPreservesExistingFields(t *testing.T) {
	tempDir := t.TempDir()
	settingsPath := filepath.Join(tempDir, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(settingsPath, []byte("{\n  \"theme\": \"dark\"\n}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	store := &GlobalSettingsStore{Path: settingsPath}
	if err := store.SaveModel(context.Background(), "claude-opus-4-1"); err != nil {
		t.Fatalf("SaveModel() error = %v", err)
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	got := string(data)
	want := "{\n  \"model\": \"claude-opus-4-1\",\n  \"theme\": \"dark\"\n}\n"
	if got != want {
		t.Fatalf("settings content = %q, want %q", got, want)
	}
}

// TestGlobalSettingsStoreSaveModelClearsOverride verifies saving the default model removes the explicit settings override.
func TestGlobalSettingsStoreSaveModelClearsOverride(t *testing.T) {
	tempDir := t.TempDir()
	settingsPath := filepath.Join(tempDir, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(settingsPath, []byte("{\n  \"model\": \"claude-opus-4-1\",\n  \"theme\": \"light\"\n}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	store := &GlobalSettingsStore{Path: settingsPath}
	if err := store.SaveModel(context.Background(), ""); err != nil {
		t.Fatalf("SaveModel() error = %v", err)
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	got := string(data)
	want := "{\n  \"theme\": \"light\"\n}\n"
	if got != want {
		t.Fatalf("settings content = %q, want %q", got, want)
	}
}

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

// TestGlobalSettingsStoreSaveModelCreatesFile verifies the store can create a missing global settings file for model writes.
func TestGlobalSettingsStoreSaveModelCreatesFile(t *testing.T) {
	tempDir := t.TempDir()
	settingsPath := filepath.Join(tempDir, ".claude", "settings.json")

	store := &GlobalSettingsStore{Path: settingsPath}
	if err := store.SaveModel(context.Background(), "claude-sonnet-4-5"); err != nil {
		t.Fatalf("SaveModel() error = %v", err)
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	got := string(data)
	want := "{\n  \"model\": \"claude-sonnet-4-5\"\n}\n"
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

// TestGlobalSettingsStoreSaveEffortLevelPreservesExistingFields verifies effortLevel updates keep unrelated settings content intact.
func TestGlobalSettingsStoreSaveEffortLevelPreservesExistingFields(t *testing.T) {
	tempDir := t.TempDir()
	settingsPath := filepath.Join(tempDir, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(settingsPath, []byte("{\n  \"model\": \"sonnet\"\n}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	store := &GlobalSettingsStore{Path: settingsPath}
	if err := store.SaveEffortLevel(context.Background(), "high"); err != nil {
		t.Fatalf("SaveEffortLevel() error = %v", err)
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	got := string(data)
	want := "{\n  \"effortLevel\": \"high\",\n  \"model\": \"sonnet\"\n}\n"
	if got != want {
		t.Fatalf("settings content = %q, want %q", got, want)
	}
}

// TestGlobalSettingsStoreSaveEffortLevelClearsOverride verifies auto removes the explicit effort override.
func TestGlobalSettingsStoreSaveEffortLevelClearsOverride(t *testing.T) {
	tempDir := t.TempDir()
	settingsPath := filepath.Join(tempDir, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(settingsPath, []byte("{\n  \"effortLevel\": \"medium\",\n  \"theme\": \"light\"\n}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	store := &GlobalSettingsStore{Path: settingsPath}
	if err := store.SaveEffortLevel(context.Background(), ""); err != nil {
		t.Fatalf("SaveEffortLevel() error = %v", err)
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	got := string(data)
	want := "{\n  \"theme\": \"light\"\n}\n"
	if got != want {
		t.Fatalf("settings content = %q, want %q", got, want)
	}
}

// TestGlobalSettingsStoreSaveFastModePreservesExistingFields verifies fastMode updates keep unrelated settings content intact.
func TestGlobalSettingsStoreSaveFastModePreservesExistingFields(t *testing.T) {
	tempDir := t.TempDir()
	settingsPath := filepath.Join(tempDir, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(settingsPath, []byte("{\n  \"theme\": \"dark\"\n}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	store := &GlobalSettingsStore{Path: settingsPath}
	if err := store.SaveFastMode(context.Background(), true); err != nil {
		t.Fatalf("SaveFastMode() error = %v", err)
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	got := string(data)
	want := "{\n  \"fastMode\": true,\n  \"theme\": \"dark\"\n}\n"
	if got != want {
		t.Fatalf("settings content = %q, want %q", got, want)
	}
}

// TestGlobalSettingsStoreSaveFastModeClearsOverride verifies disabling fast mode removes the explicit settings key.
func TestGlobalSettingsStoreSaveFastModeClearsOverride(t *testing.T) {
	tempDir := t.TempDir()
	settingsPath := filepath.Join(tempDir, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(settingsPath, []byte("{\n  \"fastMode\": true,\n  \"theme\": \"light\"\n}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	store := &GlobalSettingsStore{Path: settingsPath}
	if err := store.SaveFastMode(context.Background(), false); err != nil {
		t.Fatalf("SaveFastMode() error = %v", err)
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	got := string(data)
	want := "{\n  \"theme\": \"light\"\n}\n"
	if got != want {
		t.Fatalf("settings content = %q, want %q", got, want)
	}
}
