package plugin

import (
	"os"
	"path/filepath"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// defaultPluginsDirName is the default subdirectory under the config home for
// plugin storage.
const defaultPluginsDirName = "plugins"

// GetPluginsDir returns the path to the plugins directory. It respects the
// CLAUDE_CODE_PLUGIN_CACHE_DIR environment variable as an override; otherwise
// it returns ~/.claude/plugins.
func GetPluginsDir() string {
	if envDir := os.Getenv("CLAUDE_CODE_PLUGIN_CACHE_DIR"); envDir != "" {
		return os.ExpandEnv(envDir)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		logger.DebugCF("plugin.discovery", "failed to get home dir", map[string]any{
			"error": err.Error(),
		})
		return filepath.Join(".claude", defaultPluginsDirName)
	}
	return filepath.Join(home, ".claude", defaultPluginsDirName)
}

// pluginManifestNames lists the recognised manifest file names in priority
// order. plugin.json takes precedence over package.json.
var pluginManifestNames = []string{"plugin.json", "package.json"}

// DiscoverPluginDirs scans the given base directory for subdirectories that
// contain a plugin manifest file (plugin.json or package.json). It returns
// absolute paths to the discovered plugin root directories.
//
// Non-directory entries and directories without a recognised manifest are
// silently skipped.
func DiscoverPluginDirs(baseDir string) ([]string, error) {
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			logger.DebugCF("plugin.discovery", "plugins dir does not exist", map[string]any{
				"dir": baseDir,
			})
			return nil, nil
		}
		return nil, err
	}

	var dirs []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pluginDir := filepath.Join(baseDir, entry.Name())
		if !hasManifestFile(pluginDir) {
			continue
		}
		dirs = append(dirs, pluginDir)
	}

	logger.DebugCF("plugin.discovery", "discovered plugin dirs", map[string]any{
		"dir":   baseDir,
		"count": len(dirs),
	})
	return dirs, nil
}

// DiscoverAllPluginDirs discovers plugins from all standard locations:
// 1. Project-level: <cwd>/.claude/plugins/
// 2. User-level: GetPluginsDir()
// 3. Built-in: <executable>/plugins/ (future use)
//
// Results from different sources are merged; later sources do not override
// earlier ones. Duplicate plugin names are resolved at registration time.
func DiscoverAllPluginDirs() []string {
	var all []string

	// Project-level plugins.
	cwd, err := os.Getwd()
	if err == nil {
		projectDir := filepath.Join(cwd, ".claude", defaultPluginsDirName)
		if projectPlugins, err := DiscoverPluginDirs(projectDir); err == nil {
			all = append(all, projectPlugins...)
		} else {
			logger.DebugCF("plugin.discovery", "failed to discover project plugins", map[string]any{
				"dir":   projectDir,
				"error": err.Error(),
			})
		}
	}

	// User-level plugins.
	userDir := GetPluginsDir()
	if userPlugins, err := DiscoverPluginDirs(userDir); err == nil {
		all = append(all, userPlugins...)
	} else {
		logger.DebugCF("plugin.discovery", "failed to discover user plugins", map[string]any{
			"dir":   userDir,
			"error": err.Error(),
		})
	}

	logger.DebugCF("plugin.discovery", "all discovered plugin dirs", map[string]any{
		"count": len(all),
	})
	return all
}

// hasManifestFile returns true if the given directory contains at least one
// recognised manifest file (plugin.json or package.json).
func hasManifestFile(dir string) bool {
	for _, name := range pluginManifestNames {
		manifestPath := filepath.Join(dir, name)
		if info, err := os.Stat(manifestPath); err == nil && !info.IsDir() {
			return true
		}
	}
	return false
}
