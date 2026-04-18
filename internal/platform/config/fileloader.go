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
	OAuthAccount  struct {
		AccountUUID      string `json:"accountUuid"`
		EmailAddress     string `json:"emailAddress"`
		OrganizationUUID string `json:"organizationUuid"`
		OrganizationName string `json:"organizationName"`
	} `json:"oauthAccount"`
	Permissions struct {
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

	sourceEnvs := map[SettingSource]map[string]string{}
	loadedSettingSources := make([]string, 0, 4)
	for _, candidate := range l.settingsPathCandidates() {
		fileCfg, loaded, err := l.loadSettingsFile(candidate)
		if err != nil {
			return coreconfig.Config{}, err
		}
		if loaded {
			loadedSettingSources = append(loadedSettingSources, string(candidate.Source))
		}
		sourceEnvs[candidate.Source] = cloneStringMap(fileCfg.Env)
		cfg = coreconfig.Merge(cfg, fileCfg)
	}
	if strings.TrimSpace(l.FlagSettingsValue) != "" {
		flagCfg, err := l.loadFlagSettings(l.FlagSettingsValue)
		if err != nil {
			return coreconfig.Config{}, err
		}
		loadedSettingSources = append(loadedSettingSources, string(SettingSourceFlagSettings))
		sourceEnvs[SettingSourceFlagSettings] = cloneStringMap(flagCfg.Env)
		cfg = coreconfig.Merge(cfg, flagCfg)
	}
	cfg.LoadedSettingSources = append([]string(nil), loadedSettingSources...)
	cfg.Env = buildRuntimeSettingsEnv(sourceEnvs, isTruthySettingEnv(l.LookupEnv("CLAUDE_CODE_PROVIDER_MANAGED_BY_HOST")))

	envLookup := l.runtimeEnvLookup(cfg.Env)
	envLookupWithSource := l.runtimeEnvLookupWithSource(cfg.Env)
	envProvider, _ := envLookupWithSource("CLAUDE_CODE_PROVIDER")
	envProvider = strings.TrimSpace(envProvider)
	activeProvider := firstNonEmpty(envProvider, cfg.Provider)
	apiKey, apiKeySource := l.lookupAPIKey(activeProvider, envLookupWithSource)
	authToken, authTokenSource := l.lookupAuthToken(activeProvider, envLookupWithSource)
	apiBaseURL, apiBaseURLSource := l.lookupAPIBaseURL(activeProvider, envLookupWithSource)
	proxyURL, proxySource := firstResolvedEnvValue(envLookupWithSource, "https_proxy", "HTTPS_PROXY", "http_proxy", "HTTP_PROXY")
	additionalCACertsPath, additionalCACertsSource := envLookupWithSource("NODE_EXTRA_CA_CERTS")
	mtlsClientCertPath, mtlsClientCertSource := envLookupWithSource("CLAUDE_CODE_CLIENT_CERT")
	mtlsClientKeyPath, mtlsClientKeySource := envLookupWithSource("CLAUDE_CODE_CLIENT_KEY")
	envCfg := coreconfig.Config{
		Model:                   envLookup("CLAUDE_CODE_MODEL"),
		Theme:                   envLookup("CLAUDE_CODE_THEME"),
		EditorMode:              envLookup("CLAUDE_CODE_EDITOR_MODE"),
		Provider:                envProvider,
		APIKey:                  apiKey,
		AuthToken:               authToken,
		APIBaseURL:              apiBaseURL,
		APIKeySource:            apiKeySource,
		AuthTokenSource:         authTokenSource,
		APIBaseURLSource:        apiBaseURLSource,
		ProxyURL:                proxyURL,
		ProxySource:             proxySource,
		AdditionalCACertsPath:   additionalCACertsPath,
		AdditionalCACertsSource: additionalCACertsSource,
		MTLSClientCertPath:      mtlsClientCertPath,
		MTLSClientCertSource:    mtlsClientCertSource,
		MTLSClientKeyPath:       mtlsClientKeyPath,
		MTLSClientKeySource:     mtlsClientKeySource,
		ApprovalMode:            envLookup("CLAUDE_CODE_APPROVAL_MODE"),
		SessionDBPath:           envLookup("CLAUDE_CODE_SESSION_DB_PATH"),
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

// firstResolvedEnvValue returns the first configured environment variable together with its source label.
func firstResolvedEnvValue(lookup func(string) (string, string), keys ...string) (string, string) {
	for _, key := range keys {
		value, source := lookup(key)
		if strings.TrimSpace(value) == "" {
			continue
		}
		return value, source
	}
	return "", ""
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

// runtimeEnvLookupWithSource resolves environment variables together with a stable source label for `/status`.
func (l *FileLoader) runtimeEnvLookupWithSource(settingsEnv map[string]string) func(string) (string, string) {
	return func(key string) (string, string) {
		if settingsEnv != nil {
			if value, ok := settingsEnv[key]; ok {
				if value == "" {
					return "", ""
				}
				return value, fmt.Sprintf("%s (settings env)", key)
			}
		}
		value := l.LookupEnv(key)
		if value == "" {
			return "", ""
		}
		return value, key
	}
}

// settingsPathCandidate couples one settings source identifier with its on-disk path.
type settingsPathCandidate struct {
	// Source identifies which logical settings layer one file belongs to.
	Source SettingSource
	// Path stores the concrete settings file path that should be loaded for the source.
	Path string
}

// settingsPathCandidates returns the supported global-to-project settings lookup order together with their logical source identifiers.
func (l *FileLoader) settingsPathCandidates() []settingsPathCandidate {
	paths := make([]settingsPathCandidate, 0, 3)
	for _, source := range l.allowedSettingSources() {
		switch source {
		case SettingSourceUserSettings:
			if l.HomeDir != "" {
				paths = append(paths, settingsPathCandidate{
					Source: source,
					Path:   filepath.Join(l.HomeDir, ".claude", "settings.json"),
				})
			}
		case SettingSourceProjectSettings:
			if l.CWD != "" {
				paths = append(paths, settingsPathCandidate{
					Source: source,
					Path:   filepath.Join(l.CWD, ".claude", "settings.json"),
				})
			}
		case SettingSourceLocalSettings:
			if l.CWD != "" {
				paths = append(paths, settingsPathCandidate{
					Source: source,
					Path:   filepath.Join(l.CWD, ".claude", "settings.local.json"),
				})
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
func (l *FileLoader) loadSettingsFile(candidate settingsPathCandidate) (coreconfig.Config, bool, error) {
	data, err := os.ReadFile(candidate.Path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return coreconfig.Config{}, false, nil
		}
		return coreconfig.Config{}, false, fmt.Errorf("read settings file %s: %w", candidate.Path, err)
	}

	cfg, parseErr := parseSettingsConfig(data, candidate.Path, candidate.Source)
	if parseErr != nil {
		return coreconfig.Config{}, false, parseErr
	}
	return cfg, true, nil
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
		return parseSettingsConfig([]byte(trimmed), "--settings inline JSON", SettingSourceFlagSettings)
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
	return parseSettingsConfig(data, path, SettingSourceFlagSettings)
}

// parseSettingsConfig unmarshals the migrated settings subset and normalizes it into the runtime config model.
func parseSettingsConfig(data []byte, source string, settingSource SettingSource) (coreconfig.Config, error) {
	var parsed settingsFile
	if err := json.Unmarshal(data, &parsed); err != nil {
		return coreconfig.Config{}, fmt.Errorf("parse settings file %s: %w", source, err)
	}

	directoryEntries := coreconfig.NewAdditionalDirectoryConfigs(
		parsed.Permissions.AdditionalDirectories,
		additionalDirectorySourceFromSettingSource(settingSource),
	)

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
		OAuthAccount: coreconfig.OAuthAccountConfig{
			AccountUUID:      strings.TrimSpace(parsed.OAuthAccount.AccountUUID),
			EmailAddress:     strings.TrimSpace(parsed.OAuthAccount.EmailAddress),
			OrganizationUUID: strings.TrimSpace(parsed.OAuthAccount.OrganizationUUID),
			OrganizationName: strings.TrimSpace(parsed.OAuthAccount.OrganizationName),
		},
		Permissions: coreconfig.PermissionConfig{
			DefaultMode:                  parsed.Permissions.DefaultMode,
			Allow:                        append([]string(nil), parsed.Permissions.Allow...),
			Deny:                         append([]string(nil), parsed.Permissions.Deny...),
			Ask:                          append([]string(nil), parsed.Permissions.Ask...),
			AdditionalDirectories:        append([]string(nil), parsed.Permissions.AdditionalDirectories...),
			AdditionalDirectoryEntries:   directoryEntries,
			DisableBypassPermissionsMode: parsed.Permissions.DisableBypassPermissionsMode,
		},
	}, nil
}

// additionalDirectorySourceFromSettingSource maps one settings layer into the matching directory source label.
func additionalDirectorySourceFromSettingSource(source SettingSource) coreconfig.AdditionalDirectorySource {
	switch source {
	case SettingSourceUserSettings:
		return coreconfig.AdditionalDirectorySourceUserSettings
	case SettingSourceLocalSettings:
		return coreconfig.AdditionalDirectorySourceLocalSettings
	case SettingSourceProjectSettings:
		return coreconfig.AdditionalDirectorySourceProjectSettings
	default:
		return ""
	}
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
func (l *FileLoader) lookupAPIKey(provider string, lookup func(string) (string, string)) (string, string) {
	if value, source := lookup("CLAUDE_CODE_API_KEY"); value != "" {
		return value, source
	}

	switch coreconfig.NormalizeProvider(provider) {
	case coreconfig.ProviderAnthropic:
		return lookup("ANTHROPIC_API_KEY")
	case coreconfig.ProviderOpenAICompatible:
		return lookup("OPENAI_API_KEY")
	case coreconfig.ProviderGLM:
		for _, key := range []string{"GLM_API_KEY", "ZHIPUAI_API_KEY", "OPENAI_API_KEY"} {
			if value, source := lookup(key); value != "" {
				return value, source
			}
		}
		return "", ""
	default:
		return "", ""
	}
}

// lookupAuthToken resolves the Anthropic bearer token used by first-party account auth.
func (l *FileLoader) lookupAuthToken(provider string, lookup func(string) (string, string)) (string, string) {
	if coreconfig.NormalizeProvider(provider) != coreconfig.ProviderAnthropic {
		return "", ""
	}
	return lookup("ANTHROPIC_AUTH_TOKEN")
}

// lookupAPIBaseURL resolves the provider-specific base URL override environment variable for the active runtime provider.
func (l *FileLoader) lookupAPIBaseURL(provider string, lookup func(string) (string, string)) (string, string) {
	if value, source := lookup("CLAUDE_CODE_API_BASE_URL"); value != "" {
		return value, source
	}

	switch coreconfig.NormalizeProvider(provider) {
	case coreconfig.ProviderAnthropic:
		return lookup("ANTHROPIC_BASE_URL")
	case coreconfig.ProviderOpenAICompatible:
		for _, key := range []string{"OPENAI_BASE_URL", "OPENAI_API_BASE"} {
			if value, source := lookup(key); value != "" {
				return value, source
			}
		}
		return "", ""
	case coreconfig.ProviderGLM:
		for _, key := range []string{"GLM_BASE_URL", "ZHIPUAI_BASE_URL", "OPENAI_BASE_URL", "OPENAI_API_BASE"} {
			if value, source := lookup(key); value != "" {
				return value, source
			}
		}
		return "", ""
	default:
		return "", ""
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

// isTruthySettingEnv reports whether one host environment variable should enable a boolean runtime guard.
func isTruthySettingEnv(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
