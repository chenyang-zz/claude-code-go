package plugin

import (
	"os"
	"path/filepath"
	"regexp"
)

// sanitizeRe matches characters that are not allowed in a plugin data directory
// name. Any matched character is replaced with a hyphen.
var sanitizeRe = regexp.MustCompile(`[^a-zA-Z0-9\-_]`)

// SanitizePluginID sanitizes a plugin identifier so it can be used safely as a
// directory name. Characters outside [a-zA-Z0-9\-_] are replaced with '-'.
func SanitizePluginID(id string) string {
	return sanitizeRe.ReplaceAllString(id, "-")
}

// PluginDataDirPath returns the filesystem path to the data directory for the
// given plugin identifier, without creating the directory.
//
// The path follows the convention: {pluginsDir}/data/{sanitized-id}/
// where pluginsDir is resolved by GetPluginsDir.
func PluginDataDirPath(pluginID string) string {
	return filepath.Join(GetPluginsDir(), "data", SanitizePluginID(pluginID))
}

// GetPluginDataDir returns the filesystem path to the data directory for the
// given plugin identifier, creating the directory (and any missing parents) if
// it does not already exist.
//
// The path follows the convention: {pluginsDir}/data/{sanitized-id}/
// where pluginsDir is resolved by GetPluginsDir.
func GetPluginDataDir(pluginID string) (string, error) {
	dir := PluginDataDirPath(pluginID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}
