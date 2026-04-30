package plugin

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// UninstallPlugin removes an installed plugin by name. It deletes the plugin
// directory from the plugins directory and removes the installation record
// from installed_plugins.json.
//
// If the plugin directory does not exist, the function still attempts to clean
// up the installation record (best-effort).
func UninstallPlugin(name string, scope string, projectPath string) error {
	if name == "" {
		return fmt.Errorf("plugin name is required for uninstall")
	}

	var pluginDir string
	switch scope {
	case "user":
		pluginDir = filepath.Join(GetPluginsDir(), name)
	case "project":
		if projectPath == "" {
			return fmt.Errorf("project path is required for project-scoped uninstall")
		}
		pluginDir = filepath.Join(projectPath, ".claude", "plugins", name)
	default:
		return fmt.Errorf("unknown scope %q", scope)
	}

	// Remove the plugin directory from disk.
	if err := os.RemoveAll(pluginDir); err != nil {
		return fmt.Errorf("failed to remove plugin directory %s: %w", pluginDir, err)
	}

	// Remove the installation record from installed_plugins.json.
	store := NewInstalledPluginsStore()
	pluginID := computePluginID(name)
	if err := store.RemovePlugin(pluginID, scope, projectPath); err != nil {
		return fmt.Errorf("failed to remove plugin from installed plugins registry: %w", err)
	}

	logger.InfoCF("plugin.uninstall", "plugin uninstalled", map[string]any{
		"name":  name,
		"scope": scope,
		"path":  pluginDir,
	})
	return nil
}
