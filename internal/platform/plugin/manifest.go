package plugin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// LoadManifest reads and parses the plugin manifest from the given directory.
// It looks for plugin.json first, then falls back to package.json. Only
// metadata fields are parsed; capability definitions (hooks, commands, agents,
// MCP servers, LSP servers) are silently ignored.
//
// Returns an error if no recognised manifest file is found, the file cannot be
// read, or the JSON is malformed.
func LoadManifest(pluginDir string) (*PluginManifest, error) {
	manifestPath := findManifestFile(pluginDir)
	if manifestPath == "" {
		return nil, fmt.Errorf("no plugin manifest found in %s (expected plugin.json or package.json)", pluginDir)
	}

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest %s: %w", manifestPath, err)
	}

	var manifest PluginManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest %s: %w", manifestPath, err)
	}

	if err := ValidateManifest(&manifest); err != nil {
		return nil, fmt.Errorf("invalid manifest %s: %w", manifestPath, err)
	}

	logger.DebugCF("plugin.manifest", "loaded manifest", map[string]any{
		"plugin": manifest.Name,
		"path":   manifestPath,
	})
	return &manifest, nil
}

// findManifestFile looks for a recognised manifest file in the given directory.
// It returns the full path to the first match in priority order (plugin.json
// before package.json), or an empty string if none is found.
func findManifestFile(dir string) string {
	for _, name := range pluginManifestNames {
		path := filepath.Join(dir, name)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path
		}
	}
	return ""
}

// ValidateManifest checks that the required manifest fields are present and
// valid. Currently, only the name field is required and must not contain
// spaces.
func ValidateManifest(m *PluginManifest) error {
	if m == nil {
		return fmt.Errorf("manifest is nil")
	}
	if strings.TrimSpace(m.Name) == "" {
		return fmt.Errorf("plugin name is required and must not be empty")
	}
	if strings.Contains(m.Name, " ") {
		return fmt.Errorf("plugin name %q must not contain spaces (use kebab-case)", m.Name)
	}
	return nil
}
