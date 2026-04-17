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
	if cfg.APIBaseURL != "https://glm.example.com" {
		t.Fatalf("Load() api base url = %q, want https://glm.example.com", cfg.APIBaseURL)
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
}

// TestFileLoaderLoadSettingSourcesFilter verifies the loader only reads the disk-backed settings sources enabled by `--setting-sources`.
func TestFileLoaderLoadSettingSourcesFilter(t *testing.T) {
	tempDir := t.TempDir()
	projectDir := filepath.Join(tempDir, "project")
	homeDir := filepath.Join(tempDir, "home")

	if err := os.MkdirAll(filepath.Join(projectDir, ".claude"), 0o755); err != nil {
		t.Fatalf("MkdirAll(project) error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(homeDir, ".claude"), 0o755); err != nil {
		t.Fatalf("MkdirAll(home) error = %v", err)
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

	loader := NewFileLoader(projectDir, homeDir, func(string) string { return "" })
	loader.AllowedSettingSources = []SettingSource{SettingSourceProjectSettings}

	cfg, err := loader.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Model != "project-model" {
		t.Fatalf("Load() model = %q, want project-model", cfg.Model)
	}
}

// TestFileLoaderLoadSettingSourcesEmptyStillAppliesFlagSettings verifies disabling disk-backed settings does not suppress `--settings` overrides.
func TestFileLoaderLoadSettingSourcesEmptyStillAppliesFlagSettings(t *testing.T) {
	tempDir := t.TempDir()
	projectDir := filepath.Join(tempDir, "project")
	homeDir := filepath.Join(tempDir, "home")

	if err := os.MkdirAll(filepath.Join(projectDir, ".claude"), 0o755); err != nil {
		t.Fatalf("MkdirAll(project) error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(homeDir, ".claude"), 0o755); err != nil {
		t.Fatalf("MkdirAll(home) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(homeDir, ".claude", "settings.json"), []byte(`{"model":"home-model"}`), 0o644); err != nil {
		t.Fatalf("WriteFile(home settings) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, ".claude", "settings.json"), []byte(`{"model":"project-model"}`), 0o644); err != nil {
		t.Fatalf("WriteFile(project settings) error = %v", err)
	}

	loader := NewFileLoader(projectDir, homeDir, func(string) string { return "" })
	loader.AllowedSettingSources = []SettingSource{}
	loader.FlagSettingsValue = `{"model":"flag-model"}`

	cfg, err := loader.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Model != "flag-model" {
		t.Fatalf("Load() model = %q, want flag-model", cfg.Model)
	}
}
