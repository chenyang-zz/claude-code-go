package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

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
}

type settingsFile struct {
	Model         string `json:"model"`
	EditorMode    string `json:"editorMode"`
	Provider      string `json:"provider"`
	SessionDBPath string `json:"sessionDbPath"`
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

	envCfg := coreconfig.Config{
		Model:         l.LookupEnv("CLAUDE_CODE_MODEL"),
		EditorMode:    l.LookupEnv("CLAUDE_CODE_EDITOR_MODE"),
		Provider:      l.LookupEnv("CLAUDE_CODE_PROVIDER"),
		APIKey:        l.LookupEnv("ANTHROPIC_API_KEY"),
		APIBaseURL:    l.LookupEnv("ANTHROPIC_BASE_URL"),
		ApprovalMode:  l.LookupEnv("CLAUDE_CODE_APPROVAL_MODE"),
		SessionDBPath: l.LookupEnv("CLAUDE_CODE_SESSION_DB_PATH"),
	}
	cfg = coreconfig.Merge(cfg, envCfg)

	logger.DebugCF("runtime_config", "loaded runtime config", map[string]any{
		"provider":            cfg.Provider,
		"model":               cfg.Model,
		"editor_mode":         cfg.EditorMode,
		"has_api_key":         cfg.APIKey != "",
		"api_base_url":        cfg.APIBaseURL,
		"has_session_db_path": cfg.SessionDBPath != "",
	})

	return cfg, nil
}

// settingsPaths returns the supported global-to-project settings lookup order.
func (l *FileLoader) settingsPaths() []string {
	paths := make([]string, 0, 2)
	if l.HomeDir != "" {
		paths = append(paths, filepath.Join(l.HomeDir, ".claude", "settings.json"))
	}
	if l.CWD != "" {
		paths = append(paths, filepath.Join(l.CWD, ".claude", "settings.json"))
	}
	return paths
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

	var parsed settingsFile
	if err := json.Unmarshal(data, &parsed); err != nil {
		return coreconfig.Config{}, fmt.Errorf("parse settings file %s: %w", path, err)
	}

	return coreconfig.Config{
		Model:         parsed.Model,
		EditorMode:    coreconfig.NormalizeEditorMode(parsed.EditorMode),
		Provider:      parsed.Provider,
		ApprovalMode:  parsed.Permissions.DefaultMode,
		SessionDBPath: parsed.SessionDBPath,
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

// defaultSessionDBPath resolves the Go host's default SQLite location when a home directory is available.
func (l *FileLoader) defaultSessionDBPath() string {
	if l == nil || l.HomeDir == "" {
		return ""
	}
	return filepath.Join(l.HomeDir, DefaultSessionDBRelativePath)
}
