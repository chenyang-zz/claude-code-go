package config

import (
	"context"
	"os"
	"path/filepath"
	"strings"
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

// TestFileLoaderLoadOAuthAccount verifies the loader preserves the minimum cached oauthAccount metadata from settings.
func TestFileLoaderLoadOAuthAccount(t *testing.T) {
	tempDir := t.TempDir()
	projectDir := filepath.Join(tempDir, "project")
	homeDir := filepath.Join(tempDir, "home")

	if err := os.MkdirAll(filepath.Join(homeDir, ".claude"), 0o755); err != nil {
		t.Fatalf("MkdirAll(home) error = %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(homeDir, ".claude", "settings.json"),
		[]byte(`{"oauthAccount":{"accountUuid":"acct-123","emailAddress":"user@example.com","organizationUuid":"org-456","organizationName":"Example Org"}}`),
		0o644,
	); err != nil {
		t.Fatalf("WriteFile(home settings) error = %v", err)
	}

	loader := NewFileLoader(projectDir, homeDir, func(string) string { return "" })
	cfg, err := loader.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.OAuthAccount.AccountUUID != "acct-123" {
		t.Fatalf("Load() oauthAccount.accountUuid = %q, want acct-123", cfg.OAuthAccount.AccountUUID)
	}
	if cfg.OAuthAccount.EmailAddress != "user@example.com" {
		t.Fatalf("Load() oauthAccount.emailAddress = %q, want user@example.com", cfg.OAuthAccount.EmailAddress)
	}
	if cfg.OAuthAccount.OrganizationUUID != "org-456" {
		t.Fatalf("Load() oauthAccount.organizationUuid = %q, want org-456", cfg.OAuthAccount.OrganizationUUID)
	}
	if cfg.OAuthAccount.OrganizationName != "Example Org" {
		t.Fatalf("Load() oauthAccount.organizationName = %q, want Example Org", cfg.OAuthAccount.OrganizationName)
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
	if len(cfg.Permissions.AdditionalDirectoryEntries) != 1 {
		t.Fatalf("Load() permissions.additionalDirectoryEntries = %#v, want one entry", cfg.Permissions.AdditionalDirectoryEntries)
	}
	if cfg.Permissions.AdditionalDirectoryEntries[0].Path != "packages/local" || cfg.Permissions.AdditionalDirectoryEntries[0].Source != coreconfig.AdditionalDirectorySourceLocalSettings {
		t.Fatalf("Load() permissions.additionalDirectoryEntries[0] = %#v, want localSettings packages/local", cfg.Permissions.AdditionalDirectoryEntries[0])
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

// TestFileLoaderLoadGLMEnvFallback verifies the loader resolves GLM-specific environment variables through the provider alias.
func TestFileLoaderLoadGLMEnvFallback(t *testing.T) {
	loader := NewFileLoader(t.TempDir(), t.TempDir(), func(key string) string {
		switch key {
		case "CLAUDE_CODE_PROVIDER":
			return "zhipuai"
		case "GLM_API_KEY":
			return "glm-key"
		case "ZHIPUAI_BASE_URL":
			return "https://glm.example.com"
		default:
			return ""
		}
	})

	cfg, err := loader.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Provider != coreconfig.ProviderGLM {
		t.Fatalf("Load() provider = %q, want %q", cfg.Provider, coreconfig.ProviderGLM)
	}
	if cfg.APIKey != "glm-key" {
		t.Fatalf("Load() api key = %q, want glm-key", cfg.APIKey)
	}
	if cfg.APIKeySource != "GLM_API_KEY" {
		t.Fatalf("Load() api key source = %q, want GLM_API_KEY", cfg.APIKeySource)
	}
	if cfg.APIBaseURL != "https://glm.example.com" {
		t.Fatalf("Load() api base url = %q, want https://glm.example.com", cfg.APIBaseURL)
	}
	if cfg.APIBaseURLSource != "ZHIPUAI_BASE_URL" {
		t.Fatalf("Load() api base url source = %q, want ZHIPUAI_BASE_URL", cfg.APIBaseURLSource)
	}
}

// TestFileLoaderLoadFlagSettingsJSONOverridesFiles verifies `--settings` inline JSON merges after on-disk settings and before env.
func TestFileLoaderLoadFlagSettingsJSONOverridesFiles(t *testing.T) {
	tempDir := t.TempDir()
	projectDir := filepath.Join(tempDir, "project")
	homeDir := filepath.Join(tempDir, "home")

	if err := os.MkdirAll(filepath.Join(projectDir, ".claude"), 0o755); err != nil {
		t.Fatalf("MkdirAll(project) error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(homeDir, ".claude"), 0o755); err != nil {
		t.Fatalf("MkdirAll(home) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(homeDir, ".claude", "settings.json"), []byte(`{"model":"home-model","theme":"light"}`), 0o644); err != nil {
		t.Fatalf("WriteFile(home settings) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, ".claude", "settings.json"), []byte(`{"model":"project-model","provider":"anthropic"}`), 0o644); err != nil {
		t.Fatalf("WriteFile(project settings) error = %v", err)
	}

	loader := NewFileLoader(projectDir, homeDir, func(key string) string {
		switch key {
		case "CLAUDE_CODE_THEME":
			return "dark-ansi"
		default:
			return ""
		}
	})
	loader.FlagSettingsValue = `{"model":"flag-model","provider":"openai-compatible","permissions":{"defaultMode":"plan"}}`

	cfg, err := loader.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Model != "flag-model" {
		t.Fatalf("Load() model = %q, want flag-model", cfg.Model)
	}
	if cfg.Provider != coreconfig.ProviderOpenAICompatible {
		t.Fatalf("Load() provider = %q, want %q", cfg.Provider, coreconfig.ProviderOpenAICompatible)
	}
	if cfg.Permissions.DefaultMode != "plan" {
		t.Fatalf("Load() permissions.defaultMode = %q, want plan", cfg.Permissions.DefaultMode)
	}
	if cfg.Theme != coreconfig.ThemeSettingDarkANSI {
		t.Fatalf("Load() theme = %q, want env dark-ansi override", cfg.Theme)
	}
}

// TestFileLoaderLoadFlagSettingsFile verifies `--settings` can point at one additional settings file resolved from the working directory.
func TestFileLoaderLoadFlagSettingsFile(t *testing.T) {
	tempDir := t.TempDir()
	projectDir := filepath.Join(tempDir, "project")
	homeDir := filepath.Join(tempDir, "home")

	if err := os.MkdirAll(filepath.Join(projectDir, ".claude"), 0o755); err != nil {
		t.Fatalf("MkdirAll(project) error = %v", err)
	}
	flagPath := filepath.Join(projectDir, "extra-settings.json")
	if err := os.WriteFile(flagPath, []byte(`{"model":"flag-file-model","provider":"glm"}`), 0o644); err != nil {
		t.Fatalf("WriteFile(flag settings) error = %v", err)
	}

	loader := NewFileLoader(projectDir, homeDir, func(string) string { return "" })
	loader.FlagSettingsValue = "./extra-settings.json"

	cfg, err := loader.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Model != "flag-file-model" {
		t.Fatalf("Load() model = %q, want flag-file-model", cfg.Model)
	}
	if cfg.Provider != coreconfig.ProviderGLM {
		t.Fatalf("Load() provider = %q, want %q", cfg.Provider, coreconfig.ProviderGLM)
	}
}

// TestFileLoaderLoadFlagSettingsRejectsInvalidJSON verifies malformed inline JSON reports a stable parse error.
func TestFileLoaderLoadFlagSettingsRejectsInvalidJSON(t *testing.T) {
	loader := NewFileLoader(t.TempDir(), t.TempDir(), func(string) string { return "" })
	loader.FlagSettingsValue = `{"model":}`

	_, err := loader.Load(context.Background())
	if err == nil {
		t.Fatal("Load() error = nil, want invalid inline JSON error")
	}
	if !strings.Contains(err.Error(), "parse settings file --settings inline JSON") {
		t.Fatalf("Load() error = %q, want inline JSON parse error", err.Error())
	}
}

// TestFileLoaderLoadFlagSettingsRejectsMissingFile verifies nonexistent `--settings` files fail before env/config wiring continues.
func TestFileLoaderLoadFlagSettingsRejectsMissingFile(t *testing.T) {
	loader := NewFileLoader(t.TempDir(), t.TempDir(), func(string) string { return "" })
	loader.FlagSettingsValue = "./missing-settings.json"

	_, err := loader.Load(context.Background())
	if err == nil {
		t.Fatal("Load() error = nil, want missing flag settings file error")
	}
	if !strings.Contains(err.Error(), "read settings file") {
		t.Fatalf("Load() error = %q, want read settings file error", err.Error())
	}
}

// TestFileLoaderLoadSettingsEnvTrustModel verifies trusted settings env applies in full while project/local env is restricted to the safe allowlist.
func TestFileLoaderLoadSettingsEnvTrustModel(t *testing.T) {
	tempDir := t.TempDir()
	projectDir := filepath.Join(tempDir, "project")
	homeDir := filepath.Join(tempDir, "home")

	if err := os.MkdirAll(filepath.Join(projectDir, ".claude"), 0o755); err != nil {
		t.Fatalf("MkdirAll(project) error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(homeDir, ".claude"), 0o755); err != nil {
		t.Fatalf("MkdirAll(home) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(homeDir, ".claude", "settings.json"), []byte(`{"provider":"anthropic","env":{"CLAUDE_CODE_MODEL":"home-env-model","ANTHROPIC_API_KEY":"home-key","PATH":"/home/bin","AWS_REGION":"us-east-1"}}`), 0o644); err != nil {
		t.Fatalf("WriteFile(home settings) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, ".claude", "settings.json"), []byte(`{"provider":"openai-compatible","env":{"OPENAI_API_KEY":"project-key","AWS_REGION":"eu-west-1","SHARED":"project"}}`), 0o644); err != nil {
		t.Fatalf("WriteFile(project settings) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, ".claude", "settings.local.json"), []byte(`{"provider":"openai-compatible","env":{"CLAUDE_CODE_MODEL":"local-env-model","ANTHROPIC_BASE_URL":"https://malicious.example.com","AWS_REGION":"ap-southeast-1","LOCAL_ONLY":"1"}}`), 0o644); err != nil {
		t.Fatalf("WriteFile(local settings) error = %v", err)
	}

	loader := NewFileLoader(projectDir, homeDir, func(key string) string {
		switch key {
		case "CLAUDE_CODE_MODEL":
			return "host-model"
		case "OPENAI_API_KEY":
			return "host-openai-key"
		case "PATH":
			return "/usr/bin"
		default:
			return ""
		}
	})

	cfg, err := loader.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Model != "home-env-model" {
		t.Fatalf("Load() model = %q, want home-env-model from trusted user settings env", cfg.Model)
	}
	if cfg.Provider != coreconfig.ProviderOpenAICompatible {
		t.Fatalf("Load() provider = %q, want %q", cfg.Provider, coreconfig.ProviderOpenAICompatible)
	}
	if cfg.APIKey != "host-openai-key" {
		t.Fatalf("Load() api key = %q, want host-openai-key because untrusted project env cannot override provider credentials", cfg.APIKey)
	}
	if cfg.Env["PATH"] != "/home/bin" {
		t.Fatalf("Load() env PATH = %q, want /home/bin", cfg.Env["PATH"])
	}
	if cfg.Env["AWS_REGION"] != "ap-southeast-1" {
		t.Fatalf("Load() env AWS_REGION = %q, want ap-southeast-1 from safe local override", cfg.Env["AWS_REGION"])
	}
	if _, ok := cfg.Env["SHARED"]; ok {
		t.Fatalf("Load() env unexpectedly contains unsafe project key SHARED: %#v", cfg.Env)
	}
	if _, ok := cfg.Env["LOCAL_ONLY"]; ok {
		t.Fatalf("Load() env unexpectedly contains unsafe local key LOCAL_ONLY: %#v", cfg.Env)
	}
	if _, ok := cfg.Env["ANTHROPIC_BASE_URL"]; ok {
		t.Fatalf("Load() env unexpectedly contains dangerous untrusted base url override: %#v", cfg.Env)
	}
}

// TestFileLoaderLoadCoercesSettingsEnvValues verifies non-string settings env values are stringified before reaching runtime config.
func TestFileLoaderLoadCoercesSettingsEnvValues(t *testing.T) {
	tempDir := t.TempDir()
	projectDir := filepath.Join(tempDir, "project")
	homeDir := filepath.Join(tempDir, "home")

	if err := os.MkdirAll(filepath.Join(homeDir, ".claude"), 0o755); err != nil {
		t.Fatalf("MkdirAll(home) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(homeDir, ".claude", "settings.json"), []byte(`{"env":{"COUNT":3,"ENABLED":true,"NAME":"claude"}}`), 0o644); err != nil {
		t.Fatalf("WriteFile(home settings) error = %v", err)
	}

	loader := NewFileLoader(projectDir, homeDir, func(string) string { return "" })

	cfg, err := loader.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Env["COUNT"] != "3" {
		t.Fatalf("Load() env COUNT = %q, want 3", cfg.Env["COUNT"])
	}
	if cfg.Env["ENABLED"] != "true" {
		t.Fatalf("Load() env ENABLED = %q, want true", cfg.Env["ENABLED"])
	}
	if cfg.Env["NAME"] != "claude" {
		t.Fatalf("Load() env NAME = %q, want claude", cfg.Env["NAME"])
	}
}

// TestFileLoaderLoadPreservesHooksPolicyFields verifies hooks-related policy fields survive loader parsing.
func TestFileLoaderLoadPreservesHooksPolicyFields(t *testing.T) {
	tempDir := t.TempDir()
	projectDir := filepath.Join(tempDir, "project")
	homeDir := filepath.Join(tempDir, "home")

	if err := os.MkdirAll(filepath.Join(homeDir, ".claude"), 0o755); err != nil {
		t.Fatalf("MkdirAll(home) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(homeDir, ".claude", "settings.json"), []byte(`{"allowManagedHooksOnly":true,"allowedHttpHookUrls":["https://hooks.example.com/*"],"httpHookAllowedEnvVars":["MY_TOKEN"]}`), 0o644); err != nil {
		t.Fatalf("WriteFile(home settings) error = %v", err)
	}

	loader := NewFileLoader(projectDir, homeDir, func(string) string { return "" })

	cfg, err := loader.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if !cfg.HasAllowManagedHooksOnlySetting || !cfg.AllowManagedHooksOnly {
		t.Fatalf("Load() allowManagedHooksOnly = %v (has=%v), want true with explicit setting", cfg.AllowManagedHooksOnly, cfg.HasAllowManagedHooksOnlySetting)
	}
	if !cfg.HasAllowedHttpHookUrls || len(cfg.AllowedHttpHookUrls) != 1 || cfg.AllowedHttpHookUrls[0] != "https://hooks.example.com/*" {
		t.Fatalf("Load() allowedHttpHookUrls = %#v (has=%v), want one hooks URL", cfg.AllowedHttpHookUrls, cfg.HasAllowedHttpHookUrls)
	}
	if !cfg.HasHttpHookAllowedEnvVars || len(cfg.HttpHookAllowedEnvVars) != 1 || cfg.HttpHookAllowedEnvVars[0] != "MY_TOKEN" {
		t.Fatalf("Load() httpHookAllowedEnvVars = %#v (has=%v), want one env var", cfg.HttpHookAllowedEnvVars, cfg.HasHttpHookAllowedEnvVars)
	}
}

// TestFileLoaderLoadPreservesComplexSettingsFields verifies the loader preserves the structural settings fields added in batch-123.
func TestFileLoaderLoadPreservesComplexSettingsFields(t *testing.T) {
	tempDir := t.TempDir()
	projectDir := filepath.Join(tempDir, "project")
	homeDir := filepath.Join(tempDir, "home")

	if err := os.MkdirAll(filepath.Join(homeDir, ".claude"), 0o755); err != nil {
		t.Fatalf("MkdirAll(home) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(homeDir, ".claude", "settings.json"), []byte(`{"extraKnownMarketplaces":{"anthropic-tools":{"source":{"source":"settings","name":"anthropic-tools"}}},"sandbox":{"mode":"workspace"},"pluginConfigs":{"example@anthropic-tools":{"options":{"flag":true}}},"remote":{"defaultEnvironmentId":"env-123"},"autoUpdatesChannel":"stable","minimumVersion":"1.2.3","plansDirectory":"plans","channelsEnabled":true,"allowedChannelPlugins":[{"marketplace":"anthropic-tools","plugin":"example"}],"sshConfigs":[{"id":"prod","name":"Production","sshHost":"ops@example.com"}],"claudeMdExcludes":["**/legacy/CLAUDE.md"],"pluginTrustMessage":"Trusted internally"}`), 0o644); err != nil {
		t.Fatalf("WriteFile(home settings) error = %v", err)
	}

	loader := NewFileLoader(projectDir, homeDir, func(string) string { return "" })

	cfg, err := loader.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(cfg.ExtraKnownMarketplaces) != 1 {
		t.Fatalf("Load() extraKnownMarketplaces = %#v, want one entry", cfg.ExtraKnownMarketplaces)
	}
	if len(cfg.Sandbox) != 1 || cfg.Sandbox["mode"] != "workspace" {
		t.Fatalf("Load() sandbox = %#v, want workspace mode", cfg.Sandbox)
	}
	if len(cfg.PluginConfigs) != 1 {
		t.Fatalf("Load() pluginConfigs = %#v, want one entry", cfg.PluginConfigs)
	}
	if cfg.Remote.DefaultEnvironmentID != "env-123" {
		t.Fatalf("Load() remote.defaultEnvironmentId = %q, want env-123", cfg.Remote.DefaultEnvironmentID)
	}
	if cfg.AutoUpdatesChannel != "stable" || cfg.MinimumVersion != "1.2.3" || cfg.PlansDirectory != "plans" {
		t.Fatalf("Load() update/plans fields = %#v, want stable/1.2.3/plans", cfg)
	}
	if !cfg.ChannelsEnabled {
		t.Fatalf("Load() channelsEnabled = %v, want true", cfg.ChannelsEnabled)
	}
	if len(cfg.AllowedChannelPlugins) != 1 || cfg.AllowedChannelPlugins[0].Marketplace != "anthropic-tools" || cfg.AllowedChannelPlugins[0].Plugin != "example" {
		t.Fatalf("Load() allowedChannelPlugins = %#v, want one entry", cfg.AllowedChannelPlugins)
	}
	if len(cfg.SSHConfigs) != 1 || cfg.SSHConfigs[0].ID != "prod" || cfg.SSHConfigs[0].SSHHost != "ops@example.com" {
		t.Fatalf("Load() sshConfigs = %#v, want one SSH entry", cfg.SSHConfigs)
	}
	if len(cfg.ClaudeMdExcludes) != 1 || cfg.ClaudeMdExcludes[0] != "**/legacy/CLAUDE.md" {
		t.Fatalf("Load() claudeMdExcludes = %#v, want one exclusion", cfg.ClaudeMdExcludes)
	}
	if cfg.PluginTrustMessage != "Trusted internally" {
		t.Fatalf("Load() pluginTrustMessage = %q, want Trusted internally", cfg.PluginTrustMessage)
	}
}

// TestFileLoaderLoadFlagSettingsEnvOverridesDiskEnv verifies trusted `--settings` env entries still merge after on-disk settings.
func TestFileLoaderLoadFlagSettingsEnvOverridesDiskEnv(t *testing.T) {
	tempDir := t.TempDir()
	projectDir := filepath.Join(tempDir, "project")
	homeDir := filepath.Join(tempDir, "home")

	if err := os.MkdirAll(filepath.Join(projectDir, ".claude"), 0o755); err != nil {
		t.Fatalf("MkdirAll(project) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, ".claude", "settings.json"), []byte(`{"provider":"anthropic","env":{"CLAUDE_CODE_PROVIDER":"anthropic","ANTHROPIC_API_KEY":"disk-key"}}`), 0o644); err != nil {
		t.Fatalf("WriteFile(project settings) error = %v", err)
	}

	loader := NewFileLoader(projectDir, homeDir, func(string) string { return "" })
	loader.FlagSettingsValue = `{"provider":"anthropic","env":{"CLAUDE_CODE_PROVIDER":"glm","GLM_API_KEY":"flag-glm-key"}}`

	cfg, err := loader.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Provider != coreconfig.ProviderGLM {
		t.Fatalf("Load() provider = %q, want %q", cfg.Provider, coreconfig.ProviderGLM)
	}
	if cfg.APIKey != "flag-glm-key" {
		t.Fatalf("Load() api key = %q, want flag-glm-key", cfg.APIKey)
	}
}

// TestFileLoaderLoadHostManagedProviderFiltersSettingsEnv verifies host-managed provider mode strips provider-routing env keys from every settings source.
func TestFileLoaderLoadHostManagedProviderFiltersSettingsEnv(t *testing.T) {
	tempDir := t.TempDir()
	projectDir := filepath.Join(tempDir, "project")
	homeDir := filepath.Join(tempDir, "home")

	if err := os.MkdirAll(filepath.Join(projectDir, ".claude"), 0o755); err != nil {
		t.Fatalf("MkdirAll(project) error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(homeDir, ".claude"), 0o755); err != nil {
		t.Fatalf("MkdirAll(home) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(homeDir, ".claude", "settings.json"), []byte(`{"provider":"anthropic","env":{"CLAUDE_CODE_MODEL":"settings-model","ANTHROPIC_API_KEY":"settings-key","AWS_REGION":"us-east-1"}}`), 0o644); err != nil {
		t.Fatalf("WriteFile(home settings) error = %v", err)
	}

	loader := NewFileLoader(projectDir, homeDir, func(key string) string {
		switch key {
		case "CLAUDE_CODE_PROVIDER_MANAGED_BY_HOST":
			return "true"
		case "CLAUDE_CODE_MODEL":
			return "host-model"
		case "ANTHROPIC_API_KEY":
			return "host-key"
		default:
			return ""
		}
	})

	cfg, err := loader.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Model != "host-model" {
		t.Fatalf("Load() model = %q, want host-model when host-managed provider strips settings model override", cfg.Model)
	}
	if cfg.APIKey != "host-key" {
		t.Fatalf("Load() api key = %q, want host-key when host-managed provider strips settings credential override", cfg.APIKey)
	}
	if cfg.Env["AWS_REGION"] != "us-east-1" {
		t.Fatalf("Load() env AWS_REGION = %q, want trusted non-routing env to survive", cfg.Env["AWS_REGION"])
	}
	if _, ok := cfg.Env["CLAUDE_CODE_MODEL"]; ok {
		t.Fatalf("Load() env unexpectedly contains host-managed model override: %#v", cfg.Env)
	}
	if _, ok := cfg.Env["ANTHROPIC_API_KEY"]; ok {
		t.Fatalf("Load() env unexpectedly contains host-managed credential override: %#v", cfg.Env)
	}
}

// TestFileLoaderLoadAnthropicAuthToken verifies the loader resolves ANTHROPIC_AUTH_TOKEN for the Anthropic provider.
func TestFileLoaderLoadAnthropicAuthToken(t *testing.T) {
	loader := NewFileLoader(t.TempDir(), t.TempDir(), func(key string) string {
		switch key {
		case "ANTHROPIC_AUTH_TOKEN":
			return "test-auth-token"
		default:
			return ""
		}
	})

	cfg, err := loader.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.AuthToken != "test-auth-token" {
		t.Fatalf("Load() auth token = %q, want test-auth-token", cfg.AuthToken)
	}
	if cfg.AuthTokenSource != "ANTHROPIC_AUTH_TOKEN" {
		t.Fatalf("Load() auth token source = %q, want ANTHROPIC_AUTH_TOKEN", cfg.AuthTokenSource)
	}
}

// TestFileLoaderLoadManagedSettingsBaseAndDropIns verifies file-based managed settings merge base and alphabetical drop-ins into policySettings.
func TestFileLoaderLoadManagedSettingsBaseAndDropIns(t *testing.T) {
	tempDir := t.TempDir()
	projectDir := filepath.Join(tempDir, "project")
	homeDir := filepath.Join(tempDir, "home")
	managedDir := filepath.Join(tempDir, "managed")

	if err := os.MkdirAll(filepath.Join(managedDir, "managed-settings.d"), 0o755); err != nil {
		t.Fatalf("MkdirAll(managed drop-ins) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(managedDir, "managed-settings.json"), []byte(`{"model":"managed-base","permissions":{"additionalDirectories":["/managed/base"]}}`), 0o644); err != nil {
		t.Fatalf("WriteFile(managed base) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(managedDir, "managed-settings.d", "10-provider.json"), []byte(`{"provider":"openai-compatible"}`), 0o644); err != nil {
		t.Fatalf("WriteFile(managed drop-in provider) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(managedDir, "managed-settings.d", "20-model.json"), []byte(`{"env":{"CLAUDE_CODE_MODEL":"managed-env-model"},"permissions":{"additionalDirectories":["/managed/dropin"]}}`), 0o644); err != nil {
		t.Fatalf("WriteFile(managed drop-in model) error = %v", err)
	}

	loader := NewFileLoader(projectDir, homeDir, func(string) string { return "" })
	loader.ManagedSettingsDir = managedDir

	cfg, err := loader.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Model != "managed-env-model" {
		t.Fatalf("Load() model = %q, want managed-env-model", cfg.Model)
	}
	if cfg.Provider != coreconfig.ProviderOpenAICompatible {
		t.Fatalf("Load() provider = %q, want %q", cfg.Provider, coreconfig.ProviderOpenAICompatible)
	}
	if len(cfg.LoadedSettingSources) != 1 || cfg.LoadedSettingSources[0] != string(SettingSourcePolicySettings) {
		t.Fatalf("Load() loaded setting sources = %#v, want [policySettings]", cfg.LoadedSettingSources)
	}
	if cfg.PolicySettings.Origin != coreconfig.PolicySettingsOriginFile || !cfg.PolicySettings.HasBaseFile || !cfg.PolicySettings.HasDropIns {
		t.Fatalf("Load() policy settings = %#v, want file origin with base+drop-ins", cfg.PolicySettings)
	}
	if len(cfg.Permissions.AdditionalDirectoryEntries) != 1 {
		t.Fatalf("Load() permissions.additionalDirectoryEntries = %#v, want one managed entry", cfg.Permissions.AdditionalDirectoryEntries)
	}
	entry := cfg.Permissions.AdditionalDirectoryEntries[0]
	if entry.Path != "/managed/dropin" || entry.Source != coreconfig.AdditionalDirectorySourcePolicySettings {
		t.Fatalf("Load() permissions.additionalDirectoryEntries[0] = %#v, want managed drop-in path with policySettings source", entry)
	}
}

// TestFileLoaderLoadSettingSourcesFilter verifies the loader only reads the disk-backed settings sources enabled by `--setting-sources`.
func TestFileLoaderLoadSettingSourcesFilter(t *testing.T) {
	tempDir := t.TempDir()
	projectDir := filepath.Join(tempDir, "project")
	homeDir := filepath.Join(tempDir, "home")
	managedDir := filepath.Join(tempDir, "managed")

	if err := os.MkdirAll(filepath.Join(projectDir, ".claude"), 0o755); err != nil {
		t.Fatalf("MkdirAll(project) error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(homeDir, ".claude"), 0o755); err != nil {
		t.Fatalf("MkdirAll(home) error = %v", err)
	}
	if err := os.MkdirAll(managedDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(managed) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(homeDir, ".claude", "settings.json"), []byte(`{"model":"home-model"}`), 0o644); err != nil {
		t.Fatalf("WriteFile(home settings) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, ".claude", "settings.json"), []byte(`{"model":"project-model"}`), 0o644); err != nil {
		t.Fatalf("WriteFile(project settings) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, ".claude", "settings.local.json"), []byte(`{"model":"local-model"}`), 0o644); err != nil {
		t.Fatalf("WriteFile(local settings) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(managedDir, "managed-settings.json"), []byte(`{"model":"managed-model"}`), 0o644); err != nil {
		t.Fatalf("WriteFile(managed settings) error = %v", err)
	}

	loader := NewFileLoader(projectDir, homeDir, func(string) string { return "" })
	loader.ManagedSettingsDir = managedDir
	loader.AllowedSettingSources = []SettingSource{SettingSourceProjectSettings}

	cfg, err := loader.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Model != "managed-model" {
		t.Fatalf("Load() model = %q, want managed-model because policySettings still participates", cfg.Model)
	}
	if len(cfg.LoadedSettingSources) != 2 || cfg.LoadedSettingSources[0] != string(SettingSourceProjectSettings) || cfg.LoadedSettingSources[1] != string(SettingSourcePolicySettings) {
		t.Fatalf("Load() loaded setting sources = %#v, want [projectSettings policySettings]", cfg.LoadedSettingSources)
	}
}

// TestFileLoaderLoadSettingSourcesEmptyStillAppliesPolicyAndFlagSettings verifies disabling user/project/local sources does not suppress policySettings or `--settings`.
func TestFileLoaderLoadSettingSourcesEmptyStillAppliesPolicyAndFlagSettings(t *testing.T) {
	tempDir := t.TempDir()
	projectDir := filepath.Join(tempDir, "project")
	homeDir := filepath.Join(tempDir, "home")
	managedDir := filepath.Join(tempDir, "managed")

	if err := os.MkdirAll(filepath.Join(projectDir, ".claude"), 0o755); err != nil {
		t.Fatalf("MkdirAll(project) error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(homeDir, ".claude"), 0o755); err != nil {
		t.Fatalf("MkdirAll(home) error = %v", err)
	}
	if err := os.MkdirAll(managedDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(managed) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(homeDir, ".claude", "settings.json"), []byte(`{"model":"home-model"}`), 0o644); err != nil {
		t.Fatalf("WriteFile(home settings) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, ".claude", "settings.json"), []byte(`{"model":"project-model"}`), 0o644); err != nil {
		t.Fatalf("WriteFile(project settings) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(managedDir, "managed-settings.json"), []byte(`{"model":"managed-model"}`), 0o644); err != nil {
		t.Fatalf("WriteFile(managed settings) error = %v", err)
	}

	loader := NewFileLoader(projectDir, homeDir, func(string) string { return "" })
	loader.ManagedSettingsDir = managedDir
	loader.AllowedSettingSources = []SettingSource{}
	loader.FlagSettingsValue = `{"model":"flag-model"}`

	cfg, err := loader.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Model != "flag-model" {
		t.Fatalf("Load() model = %q, want flag-model", cfg.Model)
	}
	if len(cfg.LoadedSettingSources) != 2 || cfg.LoadedSettingSources[0] != string(SettingSourcePolicySettings) || cfg.LoadedSettingSources[1] != string(SettingSourceFlagSettings) {
		t.Fatalf("Load() loaded setting sources = %#v, want [policySettings flagSettings]", cfg.LoadedSettingSources)
	}
}

// TestFileLoaderLoadTracksSettingsEnvCredentialSource verifies trusted settings env values report their source labels.
func TestFileLoaderLoadTracksSettingsEnvCredentialSource(t *testing.T) {
	tempDir := t.TempDir()
	projectDir := filepath.Join(tempDir, "project")
	homeDir := filepath.Join(tempDir, "home")

	if err := os.MkdirAll(filepath.Join(homeDir, ".claude"), 0o755); err != nil {
		t.Fatalf("MkdirAll(home) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(homeDir, ".claude", "settings.json"), []byte(`{"provider":"anthropic","env":{"ANTHROPIC_API_KEY":"settings-key","ANTHROPIC_BASE_URL":"https://settings.example.com"}}`), 0o644); err != nil {
		t.Fatalf("WriteFile(home settings) error = %v", err)
	}

	loader := NewFileLoader(projectDir, homeDir, func(string) string { return "" })

	cfg, err := loader.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.APIKeySource != "ANTHROPIC_API_KEY (settings env)" {
		t.Fatalf("Load() api key source = %q, want ANTHROPIC_API_KEY (settings env)", cfg.APIKeySource)
	}
	if cfg.APIBaseURLSource != "ANTHROPIC_BASE_URL (settings env)" {
		t.Fatalf("Load() api base url source = %q, want ANTHROPIC_BASE_URL (settings env)", cfg.APIBaseURLSource)
	}
	if len(cfg.LoadedSettingSources) != 1 || cfg.LoadedSettingSources[0] != string(SettingSourceUserSettings) {
		t.Fatalf("Load() loaded setting sources = %#v, want [userSettings]", cfg.LoadedSettingSources)
	}
}

// TestFileLoaderLoadTracksTransportDiagnostics verifies proxy and TLS diagnostics are surfaced from runtime environment resolution.
func TestFileLoaderLoadTracksTransportDiagnostics(t *testing.T) {
	loader := NewFileLoader(t.TempDir(), t.TempDir(), func(key string) string {
		switch key {
		case "https_proxy":
			return "http://lowercase-proxy.internal:8080"
		case "HTTPS_PROXY":
			return "http://uppercase-proxy.internal:8080"
		case "NODE_EXTRA_CA_CERTS":
			return "/etc/ssl/custom.pem"
		case "CLAUDE_CODE_CLIENT_CERT":
			return "/etc/ssl/client.pem"
		case "CLAUDE_CODE_CLIENT_KEY":
			return "/etc/ssl/client-key.pem"
		default:
			return ""
		}
	})

	cfg, err := loader.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.ProxyURL != "http://lowercase-proxy.internal:8080" {
		t.Fatalf("Load() proxy url = %q, want lowercase proxy", cfg.ProxyURL)
	}
	if cfg.ProxySource != "https_proxy" {
		t.Fatalf("Load() proxy source = %q, want https_proxy", cfg.ProxySource)
	}
	if cfg.AdditionalCACertsPath != "/etc/ssl/custom.pem" {
		t.Fatalf("Load() additional ca certs path = %q, want /etc/ssl/custom.pem", cfg.AdditionalCACertsPath)
	}
	if cfg.MTLSClientCertPath != "/etc/ssl/client.pem" {
		t.Fatalf("Load() mTLS client cert path = %q, want /etc/ssl/client.pem", cfg.MTLSClientCertPath)
	}
	if cfg.MTLSClientKeyPath != "/etc/ssl/client-key.pem" {
		t.Fatalf("Load() mTLS client key path = %q, want /etc/ssl/client-key.pem", cfg.MTLSClientKeyPath)
	}
}

func TestFileLoaderLoadRejectsInvalidHooksConfig(t *testing.T) {
	tempDir := t.TempDir()
	projectDir := filepath.Join(tempDir, "project")
	homeDir := filepath.Join(tempDir, "home")

	if err := os.MkdirAll(filepath.Join(projectDir, ".claude"), 0o755); err != nil {
		t.Fatalf("MkdirAll(project) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, ".claude", "settings.json"), []byte(`{"hooks":{"Stop":"not-an-array"}}`), 0o644); err != nil {
		t.Fatalf("WriteFile(project settings) error = %v", err)
	}

	loader := NewFileLoader(projectDir, homeDir, func(string) string { return "" })

	_, err := loader.Load(context.Background())
	if err == nil {
		t.Fatal("Load() error = nil, want invalid hooks error")
	}
	if !strings.Contains(err.Error(), "parse hooks for event Stop") {
		t.Fatalf("Load() error = %q, want hook parse failure", err.Error())
	}
}
