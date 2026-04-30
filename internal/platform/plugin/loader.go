package plugin

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// PluginLoader loads installed plugins from the InstalledPluginsStore,
// resolves their manifests, and auto-detects capability directories.
type PluginLoader struct {
	store *InstalledPluginsStore
}

// NewPluginLoader creates a new PluginLoader backed by the given store.
func NewPluginLoader(store *InstalledPluginsStore) *PluginLoader {
	return &PluginLoader{store: store}
}

// LoadAll loads all installed plugins from the store, parses their manifests,
// auto-detects capability directories, and returns a PluginLoadResult.
//
// Plugins whose install paths no longer exist on disk are reported as
// "plugin-cache-miss" errors. Plugins whose manifests cannot be loaded are
// reported as "manifest-load-error" errors. A failure to parse hooks.json
// is reported as a "hook-load-failed" error but does not prevent the plugin
// from being loaded.
func (l *PluginLoader) LoadAll() (*PluginLoadResult, error) {
	installed, err := l.store.ListAll()
	if err != nil {
		return nil, fmt.Errorf("failed to list installed plugins: %w", err)
	}

	result := &PluginLoadResult{
		Enabled:  make([]*LoadedPlugin, 0),
		Disabled: make([]*LoadedPlugin, 0),
		Errors:   make([]*PluginError, 0),
	}

	for pluginID, entries := range installed {
		for _, entry := range entries {
			l.loadEntry(pluginID, entry, result)
		}
	}

	logger.DebugCF("plugin.loader", "loaded plugins", map[string]any{
		"enabled": len(result.Enabled),
		"errors":  len(result.Errors),
	})
	return result, nil
}

// loadEntry loads a single PluginInstallEntry and appends the resulting
// LoadedPlugin or errors to the PluginLoadResult.
func (l *PluginLoader) loadEntry(pluginID string, entry PluginInstallEntry, result *PluginLoadResult) {
	// Skip entries whose install path no longer exists on disk.
	if _, err := os.Stat(entry.InstallPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			result.Errors = append(result.Errors, &PluginError{
				Type:    "plugin-cache-miss",
				Source:  pluginID,
				Plugin:  pluginID,
				Message: fmt.Sprintf("install path does not exist: %s", entry.InstallPath),
			})
		} else {
			result.Errors = append(result.Errors, &PluginError{
				Type:    "plugin-cache-miss",
				Source:  pluginID,
				Plugin:  pluginID,
				Message: fmt.Sprintf("failed to stat install path %s: %v", entry.InstallPath, err),
			})
		}
		return
	}

	// Load the manifest from the install path.
	manifest, err := LoadManifest(entry.InstallPath)
	if err != nil {
		result.Errors = append(result.Errors, &PluginError{
			Type:    "manifest-load-error",
			Source:  pluginID,
			Plugin:  pluginID,
			Message: fmt.Sprintf("failed to load manifest from %s: %v", entry.InstallPath, err),
		})
		return
	}

	// Infer the source type from the install entry.
	sourceType := SourceTypePath
	if entry.GitCommitSha != "" {
		sourceType = SourceTypeGit
	}

	plugin := &LoadedPlugin{
		Name:     manifest.Name,
		Manifest: *manifest,
		Path:     entry.InstallPath,
		Source: PluginSource{
			Type:  sourceType,
			Value: entry.InstallPath,
		},
		Enabled: true,
	}

	// Auto-detect capability directories.
	detectCapabilityDirs(plugin, entry.InstallPath)

	// Load hooks.json if present.
	hooksConfig, err := loadHooksConfig(entry.InstallPath)
	if err != nil {
		result.Errors = append(result.Errors, &PluginError{
			Type:    "hook-load-failed",
			Source:  pluginID,
			Plugin:  manifest.Name,
			Message: fmt.Sprintf("failed to load hooks config from %s: %v", entry.InstallPath, err),
		})
	} else {
		plugin.HooksConfig = hooksConfig
	}

	result.Enabled = append(result.Enabled, plugin)
}

// detectCapabilityDirs checks for the presence of standard capability
// subdirectories and sets the corresponding path fields on the plugin.
func detectCapabilityDirs(plugin *LoadedPlugin, installPath string) {
	capabilities := []struct {
		subdir string
		set    func(path string)
	}{
		{"commands", func(p string) { plugin.CommandsPath = p }},
		{"skills", func(p string) { plugin.SkillsPath = p }},
		{"agents", func(p string) { plugin.AgentsPath = p }},
		{"output-styles", func(p string) { plugin.OutputStylesPath = p }},
	}

	for _, cap := range capabilities {
		dirPath := filepath.Join(installPath, cap.subdir)
		if info, err := os.Stat(dirPath); err == nil && info.IsDir() {
			cap.set(dirPath)
		}
	}
}

// loadHooksConfig reads and parses the hooks/hooks.json file from the given
// plugin directory. It returns nil, nil if the file does not exist; an error
// is returned only if the file exists but cannot be read or parsed.
func loadHooksConfig(pluginPath string) (*HooksConfig, error) {
	hooksPath := filepath.Join(pluginPath, "hooks", "hooks.json")
	data, err := os.ReadFile(hooksPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read hooks config %s: %w", hooksPath, err)
	}

	var config HooksConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse hooks config %s: %w", hooksPath, err)
	}

	return &config, nil
}
