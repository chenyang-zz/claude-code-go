package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// FileLoader reads the minimal runtime settings from project/global files and environment variables.
type FileLoader struct {
	// CWD identifies the current project directory used for project-local settings lookup.
	CWD string
	// HomeDir identifies the user home directory used for global settings lookup.
	HomeDir string
	// LookupEnv resolves environment variables so tests can supply stable inputs.
	LookupEnv func(string) string
	// FlagSettingsValue carries one optional `--settings` CLI override that should merge after on-disk settings and before env.
	FlagSettingsValue string
	// AllowedSettingSources optionally restricts which disk-backed settings files participate in config loading.
	AllowedSettingSources []SettingSource
}

type settingsFile struct {
	Model         string            `json:"model"`
	EffortLevel   *string           `json:"effortLevel"`
	FastMode      *bool             `json:"fastMode"`
	Theme         string            `json:"theme"`
	EditorMode    string            `json:"editorMode"`
	Provider      string            `json:"provider"`
	SessionDBPath string            `json:"sessionDbPath"`
	Env           map[string]string `json:"env"`
	Permissions   struct {
		DefaultMode                  string   `json:"defaultMode"`
		Allow                        []string `json:"allow"`
		Deny                         []string `json:"deny"`
		Ask                          []string `json:"ask"`
		AdditionalDirectories        []string `json:"additionalDirectories"`
		DisableBypassPermissionsMode string   `json:"disableBypassPermissionsMode"`
	} `json:"permissions"`
}

// NewFileLoader builds a minimal loader with explicit lookup roots.
func NewFileLoader(cwd, homeDir string, lookupEnv func(string) string) *FileLoader {
	if lookupEnv == nil {
		lookupEnv = os.Getenv
	}

	return &FileLoader{
		CWD:       cwd,
		HomeDir:   homeDir,
		LookupEnv: lookupEnv,
	}
}

// NewDefaultFileLoader resolves the current process working directory and home directory.
func NewDefaultFileLoader() (*FileLoader, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("resolve working directory: %w", err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve home directory: %w", err)
	}

	return NewFileLoader(cwd, homeDir, os.Getenv), nil
}

// Load merges defaults, optional settings files and environment overrides into one runtime config.
func (l *FileLoader) Load(ctx context.Context) (coreconfig.Config, error) {
	_ = ctx

	cfg := coreconfig.DefaultConfig()
	cfg.ProjectPath = l.CWD
	cfg.SessionDBPath = l.defaultSessionDBPath()

	for _, path := range l.settingsPaths() {
		fileCfg, err := l.loadSettingsFile(path)
		if err != nil {
			return coreconfig.Config{}, err
		}
		cfg = coreconfig.Merge(cfg, fileCfg)
	}
	if strings.TrimSpace(l.FlagSettingsValue) != "" {
		flagCfg, err := l.loadFlagSettings(l.FlagSettingsValue)
		if err != nil {
			return coreconfig.Config{}, err
		}
		cfg = coreconfig.Merge(cfg, flagCfg)
	}

	envLookup := l.runtimeEnvLookup(cfg.Env)
	envProvider := strings.TrimSpace(envLookup("CLAUDE_CODE_PROVIDER"))
	activeProvider := firstNonEmpty(envProvider, cfg.Provider)
	envCfg := coreconfig.Config{
		Model:         envLookup("CLAUDE_CODE_MODEL"),
		Theme:         envLookup("CLAUDE_CODE_THEME"),
		EditorMode:    envLookup("CLAUDE_CODE_EDITOR_MODE"),
		Provider:      envProvider,
		APIKey:        l.lookupAPIKey(activeProvider, envLookup),
		AuthToken:     l.lookupAuthToken(activeProvider, envLookup),
		APIBaseURL:    l.lookupAPIBaseURL(activeProvider, envLookup),
		ApprovalMode:  envLookup("CLAUDE_CODE_APPROVAL_MODE"),
		SessionDBPath: envLookup("CLAUDE_CODE_SESSION_DB_PATH"),
	}
	cfg = coreconfig.Merge(cfg, envCfg)

	logger.DebugCF("runtime_config", "loaded runtime config", map[string]any{
		"provider":            cfg.Provider,
		"model":               cfg.Model,
		"effort_level":        cfg.EffortLevel,
		"fast_mode":           cfg.FastMode,
		"has_fast_mode":       cfg.HasFastModeSetting,
		"settings_env_count":  len(cfg.Env),
		"theme":               cfg.Theme,
		"editor_mode":         cfg.EditorMode,
		"has_api_key":         cfg.APIKey != "",
		"has_auth_token":      cfg.AuthToken != "",
		"api_base_url":        cfg.APIBaseURL,
		"has_session_db_path": cfg.SessionDBPath != "",
	})

	return cfg, nil
}

// runtimeEnvLookup resolves environment variables from merged settings.env first, then falls back to the host environment.
func (l *FileLoader) runtimeEnvLookup(settingsEnv map[string]string) func(string) string {
	return func(key string) string {
		if settingsEnv != nil {
			if value, ok := settingsEnv[key]; ok {
				return value
			}
		}
		return l.LookupEnv(key)
	}
}

// settingsPaths returns the supported global-to-project settings lookup order.
func (l *FileLoader) settingsPaths() []string {
	paths := make([]string, 0, 3)
	for _, source := range l.allowedSettingSources() {
		switch source {
		case SettingSourceUserSettings:
			if l.HomeDir != "" {
				paths = append(paths, filepath.Join(l.HomeDir, ".claude", "settings.json"))
			}
		case SettingSourceProjectSettings:
			if l.CWD != "" {
				paths = append(paths, filepath.Join(l.CWD, ".claude", "settings.json"))
			}
		case SettingSourceLocalSettings:
			if l.CWD != "" {
				paths = append(paths, filepath.Join(l.CWD, ".claude", "settings.local.json"))
			}
		}
	}
	return paths
}

// allowedSettingSources resolves the explicit CLI source filter or falls back to the default disk-backed source order.
func (l *FileLoader) allowedSettingSources() []SettingSource {
	if l == nil || l.AllowedSettingSources == nil {
		return DefaultSettingSources()
	}

	allowed := make([]SettingSource, len(l.AllowedSettingSources))
	copy(allowed, l.AllowedSettingSources)
	return allowed
}

// loadSettingsFile extracts the minimal runtime fields currently consumed by the Go host.
func (l *FileLoader) loadSettingsFile(path string) (coreconfig.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return coreconfig.Config{}, nil
		}
		return coreconfig.Config{}, fmt.Errorf("read settings file %s: %w", path, err)
	}

	return parseSettingsConfig(data, path)
}

// loadFlagSettings resolves one `--settings` value as either inline JSON or an additional settings file.
func (l *FileLoader) loadFlagSettings(value string) (coreconfig.Config, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return coreconfig.Config{}, fmt.Errorf("parse --settings: empty value")
	}
	if strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}") {
		logger.DebugCF("runtime_config", "loading flag settings from inline json", map[string]any{
			"source": "flag_json",
		})
		return parseSettingsConfig([]byte(trimmed), "--settings inline JSON")
	}

	path := trimmed
	if !filepath.IsAbs(path) {
		path = filepath.Join(l.CWD, path)
	}
	path = filepath.Clean(path)
	logger.DebugCF("runtime_config", "loading flag settings from file", map[string]any{
		"source": "flag_file",
		"path":   path,
	})
	data, err := os.ReadFile(path)
	if err != nil {
		return coreconfig.Config{}, fmt.Errorf("read settings file %s: %w", path, err)
	}
	return parseSettingsConfig(data, path)
}

// parseSettingsConfig unmarshals the migrated settings subset and normalizes it into the runtime config model.
func parseSettingsConfig(data []byte, source string) (coreconfig.Config, error) {
	var parsed settingsFile
	if err := json.Unmarshal(data, &parsed); err != nil {
		return coreconfig.Config{}, fmt.Errorf("parse settings file %s: %w", source, err)
	}

	return coreconfig.Config{
		Env:                   cloneStringMap(parsed.Env),
		Model:                 parsed.Model,
		EffortLevel:           readEffortLevel(parsed.EffortLevel),
		HasEffortLevelSetting: parsed.EffortLevel != nil,
		FastMode:              readFastMode(parsed.FastMode),
		HasFastModeSetting:    parsed.FastMode != nil,
		Theme:                 coreconfig.NormalizeThemeSetting(parsed.Theme),
		EditorMode:            coreconfig.NormalizeEditorMode(parsed.EditorMode),
		Provider:              coreconfig.NormalizeProvider(parsed.Provider),
		ApprovalMode:          parsed.Permissions.DefaultMode,
		SessionDBPath:         parsed.SessionDBPath,
		Permissions: coreconfig.PermissionConfig{
			DefaultMode:                  parsed.Permissions.DefaultMode,
			Allow:                        append([]string(nil), parsed.Permissions.Allow...),
			Deny:                         append([]string(nil), parsed.Permissions.Deny...),
			Ask:                          append([]string(nil), parsed.Permissions.Ask...),
			AdditionalDirectories:        append([]string(nil), parsed.Permissions.AdditionalDirectories...),
			DisableBypassPermissionsMode: parsed.Permissions.DisableBypassPermissionsMode,
		},
	}, nil
}

// cloneStringMap copies one string map so later merges cannot mutate decoded settings data.
func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

// readEffortLevel normalizes one optional effort pointer into the runtime representation.
func readEffortLevel(value *string) string {
	if value == nil {
		return ""
	}
	return coreconfig.NormalizeEffortLevel(*value)
}

// readFastMode dereferences one optional fast-mode pointer into the runtime representation.
func readFastMode(value *bool) bool {
	if value == nil {
		return false
	}
	return *value
}

// defaultSessionDBPath resolves the Go host's default SQLite location when a home directory is available.
func (l *FileLoader) defaultSessionDBPath() string {
	if l == nil || l.HomeDir == "" {
		return ""
	}
	return filepath.Join(l.HomeDir, DefaultSessionDBRelativePath)
}

// lookupAPIKey resolves the provider-specific credential environment variable for the active runtime provider.
func (l *FileLoader) lookupAPIKey(provider string, lookup func(string) string) string {
	if value := lookup("CLAUDE_CODE_API_KEY"); value != "" {
		return value
	}

	switch coreconfig.NormalizeProvider(provider) {
	case coreconfig.ProviderAnthropic:
		return lookup("ANTHROPIC_API_KEY")
	case coreconfig.ProviderOpenAICompatible:
		return lookup("OPENAI_API_KEY")
	case coreconfig.ProviderGLM:
		return firstNonEmpty(
			lookup("GLM_API_KEY"),
			lookup("ZHIPUAI_API_KEY"),
			lookup("OPENAI_API_KEY"),
		)
	default:
		return ""
	}
}

// lookupAuthToken resolves the Anthropic bearer token used by first-party account auth.
func (l *FileLoader) lookupAuthToken(provider string, lookup func(string) string) string {
	if coreconfig.NormalizeProvider(provider) != coreconfig.ProviderAnthropic {
		return ""
	}
	return lookup("ANTHROPIC_AUTH_TOKEN")
}

// lookupAPIBaseURL resolves the provider-specific base URL override environment variable for the active runtime provider.
func (l *FileLoader) lookupAPIBaseURL(provider string, lookup func(string) string) string {
	if value := lookup("CLAUDE_CODE_API_BASE_URL"); value != "" {
		return value
	}

	switch coreconfig.NormalizeProvider(provider) {
	case coreconfig.ProviderAnthropic:
		return lookup("ANTHROPIC_BASE_URL")
	case coreconfig.ProviderOpenAICompatible:
		return lookup("OPENAI_BASE_URL")
	case coreconfig.ProviderGLM:
		return firstNonEmpty(
			lookup("GLM_BASE_URL"),
			lookup("ZHIPUAI_BASE_URL"),
			lookup("OPENAI_BASE_URL"),
		)
	default:
		return ""
	}
}

// firstNonEmpty returns the first non-empty string from left to right.
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
