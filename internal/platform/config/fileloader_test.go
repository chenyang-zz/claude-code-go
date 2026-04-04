package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestFileLoaderLoadMergesSettingsAndEnv verifies env overrides project settings while defaults fill the rest.
func TestFileLoaderLoadMergesSettingsAndEnv(t *testing.T) {
	tempDir := t.TempDir()
	projectDir := filepath.Join(tempDir, "project")
	homeDir := filepath.Join(tempDir, "home")

	if err := os.MkdirAll(filepath.Join(projectDir, ".claude"), 0o755); err != nil {
		t.Fatalf("MkdirAll(project) error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(homeDir, ".claude"), 0o755); err != nil {
		t.Fatalf("MkdirAll(home) error = %v", err)
	}

	if err := os.WriteFile(filepath.Join(homeDir, ".claude", "settings.json"), []byte(`{"provider":"anthropic","model":"home-model"}`), 0o644); err != nil {
		t.Fatalf("WriteFile(home settings) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, ".claude", "settings.json"), []byte(`{"model":"project-model","permissions":{"defaultMode":"plan"}}`), 0o644); err != nil {
		t.Fatalf("WriteFile(project settings) error = %v", err)
	}

	loader := NewFileLoader(projectDir, homeDir, func(key string) string {
		switch key {
		case "CLAUDE_CODE_MODEL":
			return "env-model"
		case "CLAUDE_CODE_APPROVAL_MODE":
			return "bypassPermissions"
		case "ANTHROPIC_API_KEY":
			return "env-key"
		default:
			return ""
		}
	})

	cfg, err := loader.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Provider != "anthropic" || cfg.Model != "env-model" || cfg.APIKey != "env-key" || cfg.ApprovalMode != "bypassPermissions" {
		t.Fatalf("Load() = %#v, want provider anthropic, model env-model, api key env-key, approval mode bypassPermissions", cfg)
	}
}
