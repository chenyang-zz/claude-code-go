package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
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

	if err := os.WriteFile(filepath.Join(homeDir, ".claude", "settings.json"), []byte(`{"provider":"anthropic","model":"home-model","effortLevel":"medium","fastMode":true,"theme":"light","sessionDbPath":"/tmp/home-session.db","editorMode":"emacs"}`), 0o644); err != nil {
		t.Fatalf("WriteFile(home settings) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, ".claude", "settings.json"), []byte(`{"model":"project-model","effortLevel":"high","fastMode":false,"permissions":{"defaultMode":"plan","allow":["Bash(ls)"],"deny":["Bash(rm -rf)"],"ask":["Edit(*)"],"additionalDirectories":["packages/app"],"disableBypassPermissionsMode":"disable"}}`), 0o644); err != nil {
		t.Fatalf("WriteFile(project settings) error = %v", err)
	}

	loader := NewFileLoader(projectDir, homeDir, func(key string) string {
		switch key {
		case "CLAUDE_CODE_MODEL":
			return "env-model"
		case "CLAUDE_CODE_THEME":
			return "dark-ansi"
		case "CLAUDE_CODE_APPROVAL_MODE":
			return "bypassPermissions"
		case "ANTHROPIC_API_KEY":
			return "env-key"
		case "CLAUDE_CODE_SESSION_DB_PATH":
			return "/tmp/env-session.db"
		default:
			return ""
		}
	})

	cfg, err := loader.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Provider != "anthropic" || cfg.Model != "env-model" || cfg.Theme != coreconfig.ThemeSettingDarkANSI || cfg.APIKey != "env-key" || cfg.ApprovalMode != "bypassPermissions" || cfg.SessionDBPath != "/tmp/env-session.db" {
		t.Fatalf("Load() = %#v, want provider anthropic, model env-model, theme dark-ansi, api key env-key, approval mode bypassPermissions, session db /tmp/env-session.db", cfg)
	}
	if cfg.EditorMode != coreconfig.EditorModeNormal {
		t.Fatalf("Load() editor mode = %q, want %q", cfg.EditorMode, coreconfig.EditorModeNormal)
	}
	if !cfg.HasEffortLevelSetting || cfg.EffortLevel != coreconfig.EffortLevelHigh {
		t.Fatalf("Load() effort = %q (has=%v), want high with explicit setting", cfg.EffortLevel, cfg.HasEffortLevelSetting)
	}
	if !cfg.HasFastModeSetting || cfg.FastMode {
		t.Fatalf("Load() fast mode = %v (has=%v), want explicit false project override", cfg.FastMode, cfg.HasFastModeSetting)
	}
	if cfg.ProjectPath != projectDir {
		t.Fatalf("Load() project path = %q, want %q", cfg.ProjectPath, projectDir)
	}
	if cfg.Permissions.DefaultMode != "plan" {
		t.Fatalf("Load() permissions.defaultMode = %q, want %q", cfg.Permissions.DefaultMode, "plan")
	}
	if len(cfg.Permissions.Allow) != 1 || cfg.Permissions.Allow[0] != "Bash(ls)" {
		t.Fatalf("Load() permissions.allow = %#v, want Bash(ls)", cfg.Permissions.Allow)
	}
	if len(cfg.Permissions.Deny) != 1 || cfg.Permissions.Deny[0] != "Bash(rm -rf)" {
		t.Fatalf("Load() permissions.deny = %#v, want Bash(rm -rf)", cfg.Permissions.Deny)
	}
	if len(cfg.Permissions.Ask) != 1 || cfg.Permissions.Ask[0] != "Edit(*)" {
		t.Fatalf("Load() permissions.ask = %#v, want Edit(*)", cfg.Permissions.Ask)
	}
	if len(cfg.Permissions.AdditionalDirectories) != 1 || cfg.Permissions.AdditionalDirectories[0] != "packages/app" {
		t.Fatalf("Load() permissions.additionalDirectories = %#v, want packages/app", cfg.Permissions.AdditionalDirectories)
	}
	if cfg.Permissions.DisableBypassPermissionsMode != "disable" {
		t.Fatalf("Load() permissions.disableBypassPermissionsMode = %q, want disable", cfg.Permissions.DisableBypassPermissionsMode)
	}
}

// TestFileLoaderLoadProjectThemeOverridesHome verifies project settings can override the home theme setting.
func TestFileLoaderLoadProjectThemeOverridesHome(t *testing.T) {
	tempDir := t.TempDir()
	projectDir := filepath.Join(tempDir, "project")
	homeDir := filepath.Join(tempDir, "home")

	if err := os.MkdirAll(filepath.Join(projectDir, ".claude"), 0o755); err != nil {
		t.Fatalf("MkdirAll(project) error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(homeDir, ".claude"), 0o755); err != nil {
		t.Fatalf("MkdirAll(home) error = %v", err)
	}

	if err := os.WriteFile(filepath.Join(homeDir, ".claude", "settings.json"), []byte(`{"theme":"light"}`), 0o644); err != nil {
		t.Fatalf("WriteFile(home settings) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, ".claude", "settings.json"), []byte(`{"theme":"dark-daltonized"}`), 0o644); err != nil {
		t.Fatalf("WriteFile(project settings) error = %v", err)
	}

	loader := NewFileLoader(projectDir, homeDir, func(string) string { return "" })
	cfg, err := loader.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Theme != coreconfig.ThemeSettingDarkDaltonized {
		t.Fatalf("Load() theme = %q, want %q", cfg.Theme, coreconfig.ThemeSettingDarkDaltonized)
	}
}

// TestFileLoaderLoadProjectEditorModeOverridesHome verifies project settings can override the normalized home editor mode.
func TestFileLoaderLoadProjectEditorModeOverridesHome(t *testing.T) {
	tempDir := t.TempDir()
	projectDir := filepath.Join(tempDir, "project")
	homeDir := filepath.Join(tempDir, "home")

	if err := os.MkdirAll(filepath.Join(projectDir, ".claude"), 0o755); err != nil {
		t.Fatalf("MkdirAll(project) error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(homeDir, ".claude"), 0o755); err != nil {
		t.Fatalf("MkdirAll(home) error = %v", err)
	}

	if err := os.WriteFile(filepath.Join(homeDir, ".claude", "settings.json"), []byte(`{"editorMode":"emacs"}`), 0o644); err != nil {
		t.Fatalf("WriteFile(home settings) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, ".claude", "settings.json"), []byte(`{"editorMode":"vim"}`), 0o644); err != nil {
		t.Fatalf("WriteFile(project settings) error = %v", err)
	}

	loader := NewFileLoader(projectDir, homeDir, func(string) string { return "" })
	cfg, err := loader.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.EditorMode != coreconfig.EditorModeVim {
		t.Fatalf("Load() editor mode = %q, want %q", cfg.EditorMode, coreconfig.EditorModeVim)
	}
}

// TestFileLoaderLoadLocalSettingsOverrideProject verifies project-local settings.local.json overrides repository settings for migrated fields.
func TestFileLoaderLoadLocalSettingsOverrideProject(t *testing.T) {
	tempDir := t.TempDir()
	projectDir := filepath.Join(tempDir, "project")
	homeDir := filepath.Join(tempDir, "home")

	if err := os.MkdirAll(filepath.Join(projectDir, ".claude"), 0o755); err != nil {
		t.Fatalf("MkdirAll(project) error = %v", err)
	}

	if err := os.WriteFile(filepath.Join(projectDir, ".claude", "settings.json"), []byte(`{"model":"project-model","permissions":{"additionalDirectories":["packages/shared"]}}`), 0o644); err != nil {
		t.Fatalf("WriteFile(project settings) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, ".claude", "settings.local.json"), []byte(`{"model":"local-model","permissions":{"additionalDirectories":["packages/local"]}}`), 0o644); err != nil {
		t.Fatalf("WriteFile(local settings) error = %v", err)
	}

	loader := NewFileLoader(projectDir, homeDir, func(string) string { return "" })
	cfg, err := loader.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Model != "local-model" {
		t.Fatalf("Load() model = %q, want local-model", cfg.Model)
	}
	if len(cfg.Permissions.AdditionalDirectories) != 1 || cfg.Permissions.AdditionalDirectories[0] != "packages/local" {
		t.Fatalf("Load() permissions.additionalDirectories = %#v, want packages/local", cfg.Permissions.AdditionalDirectories)
	}
}

// TestFileLoaderLoadDefaultsSessionDBPath verifies the loader derives a stable default session DB path from the home directory.
func TestFileLoaderLoadDefaultsSessionDBPath(t *testing.T) {
	tempDir := t.TempDir()
	projectDir := filepath.Join(tempDir, "project")
	homeDir := filepath.Join(tempDir, "home")

	loader := NewFileLoader(projectDir, homeDir, func(string) string { return "" })

	cfg, err := loader.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	want := filepath.Join(homeDir, DefaultSessionDBRelativePath)
	if cfg.SessionDBPath != want {
		t.Fatalf("Load() session db path = %q, want %q", cfg.SessionDBPath, want)
	}
}

// TestFileLoaderLoadProjectSettingsOverrideSessionDBPath verifies project settings can override the default path.
func TestFileLoaderLoadProjectSettingsOverrideSessionDBPath(t *testing.T) {
	tempDir := t.TempDir()
	projectDir := filepath.Join(tempDir, "project")
	homeDir := filepath.Join(tempDir, "home")

	if err := os.MkdirAll(filepath.Join(projectDir, ".claude"), 0o755); err != nil {
		t.Fatalf("MkdirAll(project) error = %v", err)
	}

	projectPath := filepath.Join(projectDir, ".claude", "settings.json")
	if err := os.WriteFile(projectPath, []byte(`{"sessionDbPath":"/tmp/project-session.db"}`), 0o644); err != nil {
		t.Fatalf("WriteFile(project settings) error = %v", err)
	}

	loader := NewFileLoader(projectDir, homeDir, func(string) string { return "" })

	cfg, err := loader.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.SessionDBPath != "/tmp/project-session.db" {
		t.Fatalf("Load() session db path = %q, want /tmp/project-session.db", cfg.SessionDBPath)
	}
}
