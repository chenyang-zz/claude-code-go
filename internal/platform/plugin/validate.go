package plugin

import (
	"fmt"
	"os"
)

// ValidateInstalledPlugin checks that an installed plugin directory has a valid
// manifest. It returns an error if the directory does not exist, does not
// contain a loadable manifest, or the manifest fails validation.
func ValidateInstalledPlugin(pluginDir string) error {
	// Verify the plugin directory exists.
	info, err := os.Stat(pluginDir)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("plugin directory %s does not exist", pluginDir)
		}
		return fmt.Errorf("failed to access plugin directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("plugin path %s is not a directory", pluginDir)
	}

	// Load and validate the manifest.
	manifest, err := LoadManifest(pluginDir)
	if err != nil {
		return fmt.Errorf("failed to load plugin manifest: %w", err)
	}

	if err := ValidateManifest(manifest); err != nil {
		return fmt.Errorf("plugin manifest validation failed: %w", err)
	}

	return nil
}
