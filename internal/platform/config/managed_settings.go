package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
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
	remoteSettingsFileName   = "remote-settings.json"
	mdmDomainName            = "com.anthropic.claudecode"
	windowsPolicyRootHKLM    = `HKLM\SOFTWARE\Policies\ClaudeCode`
	windowsPolicyRootHKCU    = `HKCU\SOFTWARE\Policies\ClaudeCode`
	windowsPolicyValueName   = "Settings"
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

// remoteManagedSettingsPath builds the remote managed settings path under the user config home.
func (l *FileLoader) remoteManagedSettingsPath() string {
	return filepath.Join(l.HomeDir, ".claude", remoteSettingsFileName)
}

// loadPolicySettings applies the minimum first-source-wins policy order currently migrated in Go:
// remote managed settings -> OS-admin managed settings (HKLM/plist) -> file-based managed settings -> OS-user managed settings (HKCU).
func (l *FileLoader) loadPolicySettings() (coreconfig.Config, bool, error) {
	remoteCfg, remoteLoaded, err := l.loadRemoteManagedSettings()
	if err != nil {
		return coreconfig.Config{}, false, err
	}
	if remoteLoaded {
		remoteCfg.PolicySettings = coreconfig.PolicySettingsConfig{
			Origin: coreconfig.PolicySettingsOriginRemote,
		}
		return remoteCfg, true, nil
	}

	osAdminCfg, osAdminLoaded, err := l.loadOSAdminManagedSettings()
	if err != nil {
		return coreconfig.Config{}, false, err
	}
	if osAdminLoaded {
		osAdminCfg.PolicySettings = coreconfig.PolicySettingsConfig{
			Origin: coreconfig.PolicySettingsOriginOSAdmin,
		}
		return osAdminCfg, true, nil
	}

	fileCfg, fileLoaded, err := l.loadManagedSettings()
	if err != nil {
		return coreconfig.Config{}, false, err
	}
	if fileLoaded {
		return fileCfg, true, nil
	}

	osUserCfg, osUserLoaded, err := l.loadOSUserManagedSettings()
	if err != nil {
		return coreconfig.Config{}, false, err
	}
	if osUserLoaded {
		osUserCfg.PolicySettings = coreconfig.PolicySettingsConfig{
			Origin: coreconfig.PolicySettingsOriginOSUser,
		}
		return osUserCfg, true, nil
	}

	return coreconfig.Config{}, false, nil
}

// loadOSAdminManagedSettings loads OS-admin managed policy settings (HKLM/plist) as the second-highest policy source.
func (l *FileLoader) loadOSAdminManagedSettings() (coreconfig.Config, bool, error) {
	override := strings.TrimSpace(l.ManagedAdminSettingsPath)
	if override != "" {
		return l.loadManagedSettingsFile(override)
	}

	switch runtime.GOOS {
	case "darwin":
		return l.loadDarwinManagedSettings()
	case "windows":
		return l.loadWindowsRegistryManagedSettings(windowsPolicyRootHKLM)
	default:
		return coreconfig.Config{}, false, nil
	}
}

// loadOSUserManagedSettings loads OS-user managed policy settings (HKCU) as the lowest-priority policy fallback.
func (l *FileLoader) loadOSUserManagedSettings() (coreconfig.Config, bool, error) {
	override := strings.TrimSpace(l.ManagedUserSettingsPath)
	if override != "" {
		return l.loadManagedSettingsFile(override)
	}
	if runtime.GOOS != "windows" {
		return coreconfig.Config{}, false, nil
	}
	return l.loadWindowsRegistryManagedSettings(windowsPolicyRootHKCU)
}

// loadDarwinManagedSettings parses managed plist preferences as JSON using plutil.
func (l *FileLoader) loadDarwinManagedSettings() (coreconfig.Config, bool, error) {
	candidates := []string{
		filepath.Join("/Library/Managed Preferences", mdmDomainName+".plist"),
	}
	for _, candidate := range candidates {
		data, err := exec.Command("/usr/bin/plutil", "-convert", "json", "-o", "-", "--", candidate).CombinedOutput()
		if err != nil {
			continue
		}
		cfg, parseErr := parseSettingsConfig(data, candidate, SettingSourcePolicySettings)
		if parseErr != nil {
			return coreconfig.Config{}, false, parseErr
		}
		if hasLoadedSettingsValues(cfg) {
			return cfg, true, nil
		}
	}
	return coreconfig.Config{}, false, nil
}

// loadWindowsRegistryManagedSettings reads one policy registry value via `reg query` and parses the JSON value payload.
func (l *FileLoader) loadWindowsRegistryManagedSettings(root string) (coreconfig.Config, bool, error) {
	data, err := exec.Command("reg", "query", root, "/v", windowsPolicyValueName).CombinedOutput()
	if err != nil {
		return coreconfig.Config{}, false, nil
	}
	jsonPayload := parseWindowsRegistrySettingsValue(string(data), windowsPolicyValueName)
	if strings.TrimSpace(jsonPayload) == "" {
		return coreconfig.Config{}, false, nil
	}
	cfg, parseErr := parseSettingsConfig([]byte(jsonPayload), root, SettingSourcePolicySettings)
	if parseErr != nil {
		return coreconfig.Config{}, false, parseErr
	}
	return cfg, hasLoadedSettingsValues(cfg), nil
}

// parseWindowsRegistrySettingsValue extracts one REG_SZ payload for the managed settings value from reg query output.
func parseWindowsRegistrySettingsValue(raw, valueName string) string {
	lines := strings.Split(raw, "\n")
	needle := strings.ToLower(valueName)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		if !strings.HasPrefix(lower, needle) {
			continue
		}
		fields := bytes.Fields([]byte(trimmed))
		if len(fields) < 3 {
			continue
		}
		valueBytes := bytes.Join(fields[2:], []byte(" "))
		return strings.TrimSpace(string(valueBytes))
	}
	return ""
}

// loadRemoteManagedSettings loads one optional remote managed settings file.
func (l *FileLoader) loadRemoteManagedSettings() (coreconfig.Config, bool, error) {
	path := l.remoteManagedSettingsPath()
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
		len(cfg.EnabledPlugins) > 0 ||
		cfg.StatusLine.Type != "" ||
		cfg.StatusLine.Command != "" ||
		cfg.StatusLine.Padding != nil ||
		len(cfg.ExtraKnownMarketplaces) > 0 ||
		len(cfg.StrictKnownMarketplaces) > 0 ||
		len(cfg.BlockedMarketplaces) > 0 ||
		cfg.ForceLoginMethod != "" ||
		cfg.ForceLoginOrgUUID != "" ||
		cfg.OtelHeadersHelper != "" ||
		cfg.OutputStyle != "" ||
		cfg.Language != "" ||
		cfg.SkipWebFetchPreflight ||
		len(cfg.Sandbox) > 0 ||
		cfg.Agent != "" ||
		len(cfg.CompanyAnnouncements) > 0 ||
		len(cfg.PluginConfigs) > 0 ||
		cfg.Remote.DefaultEnvironmentID != "" ||
		cfg.AutoUpdatesChannel != "" ||
		cfg.MinimumVersion != "" ||
		cfg.PlansDirectory != "" ||
		cfg.ChannelsEnabled ||
		len(cfg.AllowedChannelPlugins) > 0 ||
		len(cfg.SSHConfigs) > 0 ||
		len(cfg.ClaudeMdExcludes) > 0 ||
		cfg.PluginTrustMessage != "" ||
		len(cfg.Permissions.Allow) > 0 ||
		len(cfg.Permissions.Deny) > 0 ||
		len(cfg.Permissions.Ask) > 0 ||
		len(cfg.Permissions.AdditionalDirectories) > 0 ||
		len(cfg.Permissions.AdditionalDirectoryEntries) > 0 ||
		cfg.Permissions.DisableBypassPermissionsMode != ""
}
