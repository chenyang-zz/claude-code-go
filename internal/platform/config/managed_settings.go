package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const (
	managedSettingsFileName  = "managed-settings.json"
	managedSettingsDropInDir = "managed-settings.d"
)

// defaultManagedSettingsDir resolves the platform-specific root used for file-based managed settings.
func defaultManagedSettingsDir() string {
	switch runtime.GOOS {
	case "darwin":
		return "/Library/Application Support/ClaudeCode"
	case "windows":
		return `C:\Program Files\ClaudeCode`
	default:
		return "/etc/claude-code"
	}
}

// managedSettingsDir resolves the effective managed settings root, allowing tests to inject a stable directory.
func (l *FileLoader) managedSettingsDir() string {
	if l != nil && strings.TrimSpace(l.ManagedSettingsDir) != "" {
		return filepath.Clean(l.ManagedSettingsDir)
	}
	return defaultManagedSettingsDir()
}

// managedSettingsFilePath builds the managed-settings.json path under one managed settings root.
func managedSettingsFilePath(root string) string {
	return filepath.Join(root, managedSettingsFileName)
}

// managedSettingsDropInPath builds the managed-settings.d path under one managed settings root.
func managedSettingsDropInPath(root string) string {
	return filepath.Join(root, managedSettingsDropInDir)
}

// loadManagedSettings loads file-based managed settings and their minimum source metadata.
func (l *FileLoader) loadManagedSettings() (coreconfig.Config, bool, error) {
	root := l.managedSettingsDir()
	basePath := managedSettingsFilePath(root)
	baseCfg, baseLoaded, err := l.loadManagedSettingsFile(basePath)
	if err != nil {
		return coreconfig.Config{}, false, err
	}

	dropInDir := managedSettingsDropInPath(root)
	dropInNames, err := discoverManagedDropIns(dropInDir)
	if err != nil {
		return coreconfig.Config{}, false, err
	}

	merged := coreconfig.Config{}
	if baseLoaded {
		merged = coreconfig.Merge(merged, baseCfg)
	}
	for _, name := range dropInNames {
		dropInPath := filepath.Join(dropInDir, name)
		dropInCfg, dropInLoaded, loadErr := l.loadManagedSettingsFile(dropInPath)
		if loadErr != nil {
			return coreconfig.Config{}, false, loadErr
		}
		if !dropInLoaded {
			continue
		}
		merged = coreconfig.Merge(merged, dropInCfg)
	}

	if !baseLoaded && len(dropInNames) == 0 {
		return coreconfig.Config{}, false, nil
	}

	if !hasLoadedSettingsValues(merged) {
		return coreconfig.Config{}, false, nil
	}

	merged.PolicySettings = coreconfig.PolicySettingsConfig{
		Origin:      coreconfig.PolicySettingsOriginFile,
		HasBaseFile: baseLoaded,
		HasDropIns:  len(dropInNames) > 0,
	}
	logger.DebugCF("runtime_config", "loaded managed file settings", map[string]any{
		"root":          root,
		"has_base_file": baseLoaded,
		"drop_in_count": len(dropInNames),
	})
	return merged, true, nil
}

// loadManagedSettingsFile loads one optional managed settings file and reports whether it contained migrated keys.
func (l *FileLoader) loadManagedSettingsFile(path string) (coreconfig.Config, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return coreconfig.Config{}, false, nil
		}
		return coreconfig.Config{}, false, fmt.Errorf("read settings file %s: %w", path, err)
	}

	cfg, parseErr := parseSettingsConfig(data, path, SettingSourcePolicySettings)
	if parseErr != nil {
		return coreconfig.Config{}, false, parseErr
	}
	return cfg, hasLoadedSettingsValues(cfg), nil
}

// discoverManagedDropIns returns the sorted managed drop-in filenames that match the source-compatible JSON filter.
func discoverManagedDropIns(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read managed settings directory %s: %w", dir, err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || strings.HasPrefix(name, ".") || !strings.HasSuffix(name, ".json") {
			continue
		}
		names = append(names, name)
	}
	slices.Sort(names)
	return names, nil
}

// hasLoadedSettingsValues reports whether one parsed settings config contained any migrated keys.
func hasLoadedSettingsValues(cfg coreconfig.Config) bool {
	return cfg.Model != "" ||
		cfg.HasEffortLevelSetting ||
		cfg.HasFastModeSetting ||
		cfg.Theme != "" ||
		cfg.EditorMode != "" ||
		cfg.Provider != "" ||
		cfg.ApprovalMode != "" ||
		cfg.SessionDBPath != "" ||
		len(cfg.Env) > 0 ||
		cfg.OAuthAccount.AccountUUID != "" ||
		cfg.OAuthAccount.EmailAddress != "" ||
		cfg.OAuthAccount.OrganizationUUID != "" ||
		cfg.OAuthAccount.OrganizationName != "" ||
		cfg.HasAllowManagedHooksOnlySetting ||
		cfg.HasAllowedHttpHookUrls ||
		cfg.HasHttpHookAllowedEnvVars ||
		len(cfg.Permissions.Allow) > 0 ||
		len(cfg.Permissions.Deny) > 0 ||
		len(cfg.Permissions.Ask) > 0 ||
		len(cfg.Permissions.AdditionalDirectories) > 0 ||
		len(cfg.Permissions.AdditionalDirectoryEntries) > 0 ||
		cfg.Permissions.DisableBypassPermissionsMode != ""
}
