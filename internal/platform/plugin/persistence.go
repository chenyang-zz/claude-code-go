package plugin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// PluginInstallEntry holds the installation metadata for a single plugin
// installation, stored in installed_plugins.json.
type PluginInstallEntry struct {
	// Scope is the installation scope: "user" or "project".
	Scope string `json:"scope"`
	// InstallPath is the absolute filesystem path to the installed plugin.
	InstallPath string `json:"installPath"`
	// Version is the resolved version string at install time.
	Version string `json:"version"`
	// InstalledAt is the ISO 8601 timestamp of the initial installation.
	InstalledAt string `json:"installedAt"`
	// LastUpdated is the ISO 8601 timestamp of the most recent update.
	LastUpdated string `json:"lastUpdated"`
	// GitCommitSha is the git commit SHA at install time, if available.
	GitCommitSha string `json:"gitCommitSha,omitempty"`
	// ProjectPath is the project directory for project-scoped installations.
	ProjectPath string `json:"projectPath,omitempty"`
}

// InstalledPluginsFile is the top-level structure for installed_plugins.json.
// It uses V2 format (version: 2) where each plugin ID maps to an array of
// installations, supporting multiple scopes.
type InstalledPluginsFile struct {
	Version int                          `json:"version"`
	Plugins map[string][]PluginInstallEntry `json:"plugins"`
}

// InstalledPluginsStore manages the installed_plugins.json file, providing
// thread-safe read/write access with file locking.
type InstalledPluginsStore struct {
	mu       sync.Mutex
	filePath string
}

// NewInstalledPluginsStore creates a new store backed by the installed_plugins.json
// file in the plugins directory.
func NewInstalledPluginsStore() *InstalledPluginsStore {
	return &InstalledPluginsStore{
		filePath: filepath.Join(GetPluginsDir(), "installed_plugins.json"),
	}
}

// Load reads the installed plugins data from disk. If the file does not exist,
// an empty store is returned without error.
func (s *InstalledPluginsStore) Load() (*InstalledPluginsFile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.loadLocked()
}

// loadLocked performs the actual file read without acquiring the lock.
func (s *InstalledPluginsStore) loadLocked() (*InstalledPluginsFile, error) {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return &InstalledPluginsFile{
				Version: 2,
				Plugins: make(map[string][]PluginInstallEntry),
			}, nil
		}
		return nil, fmt.Errorf("failed to read installed plugins file: %w", err)
	}

	var file InstalledPluginsFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("failed to parse installed plugins file: %w", err)
	}

	// Default to V2 format for compatibility.
	if file.Version == 0 {
		file.Version = 2
	}
	if file.Plugins == nil {
		file.Plugins = make(map[string][]PluginInstallEntry)
	}

	return &file, nil
}

// Save writes the installed plugins data to disk, creating the plugins
// directory if it does not exist.
func (s *InstalledPluginsStore) Save(file *InstalledPluginsFile) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.saveLocked(file)
}

// saveLocked performs the actual file write without acquiring the lock.
func (s *InstalledPluginsStore) saveLocked(file *InstalledPluginsFile) error {
	if file.Version == 0 {
		file.Version = 2
	}

	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal installed plugins data: %w", err)
	}

	// Ensure the plugins directory exists.
	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create plugins directory: %w", err)
	}

	if err := os.WriteFile(s.filePath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write installed plugins file: %w", err)
	}

	logger.DebugCF("plugin.persistence", "installed plugins file saved", map[string]any{
		"path":    s.filePath,
		"plugins": len(file.Plugins),
	})
	return nil
}

// AddPlugin adds or updates a plugin installation entry in the store.
//
// It reads the current file, adds or replaces the entry for the given plugin ID
// and scope, then writes back. If an entry with the same plugin ID and scope
// already exists, it is updated; otherwise a new entry is appended.
func (s *InstalledPluginsStore) AddPlugin(pluginID string, entry PluginInstallEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	file, err := s.loadLocked()
	if err != nil {
		return err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	if entry.InstalledAt == "" {
		entry.InstalledAt = now
	}
	entry.LastUpdated = now

	installations := file.Plugins[pluginID]

	// Find and update existing entry for the same scope, or append new.
	found := false
	for i, existing := range installations {
		if existing.Scope == entry.Scope && existing.ProjectPath == entry.ProjectPath {
			// Preserve the original installation timestamp.
			entry.InstalledAt = existing.InstalledAt
			installations[i] = entry
			found = true
			break
		}
	}
	if !found {
		installations = append(installations, entry)
	}

	file.Plugins[pluginID] = installations

	return s.saveLocked(file)
}

// RemovePlugin removes a plugin installation entry from the store by plugin ID
// and scope. If no matching entry is found, this is a no-op.
//
// If the plugin has no remaining installations after removal, the plugin ID
// is deleted from the file.
func (s *InstalledPluginsStore) RemovePlugin(pluginID, scope string, projectPath string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	file, err := s.loadLocked()
	if err != nil {
		return err
	}

	installations := file.Plugins[pluginID]
	if len(installations) == 0 {
		return nil
	}

	filtered := make([]PluginInstallEntry, 0, len(installations))
	for _, entry := range installations {
		if entry.Scope == scope && entry.ProjectPath == projectPath {
			continue
		}
		filtered = append(filtered, entry)
	}

	if len(filtered) == 0 {
		delete(file.Plugins, pluginID)
	} else {
		file.Plugins[pluginID] = filtered
	}

	return s.saveLocked(file)
}

// GetPlugin returns all installation entries for a given plugin ID.
// Returns nil if the plugin is not found.
func (s *InstalledPluginsStore) GetPlugin(pluginID string) ([]PluginInstallEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	file, err := s.loadLocked()
	if err != nil {
		return nil, err
	}

	return file.Plugins[pluginID], nil
}

// ListAll returns all installed plugin IDs and their installation entries.
func (s *InstalledPluginsStore) ListAll() (map[string][]PluginInstallEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	file, err := s.loadLocked()
	if err != nil {
		return nil, err
	}

	return file.Plugins, nil
}

// computePluginID builds a plugin identifier from the plugin name. In the
// simplified format without marketplace support, this is just the plugin name.
// When marketplace support is added, this will become "name@marketplace".
func computePluginID(manifestName string) string {
	return manifestName
}
