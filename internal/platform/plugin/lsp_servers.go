package plugin

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// ExtractLspServers extracts LSP server configurations from a plugin.
// It reads from the plugin's .lsp.json file. Overlays from manifest.lspServers
// are deferred — the PluginManifest struct does not yet carry an lspServers
// field.
func ExtractLspServers(plugin *LoadedPlugin) ([]*LspServerConfig, error) {
	configs, err := LoadLspServersFromFile(filepath.Join(plugin.Path, ".lsp.json"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to load LSP servers for plugin %s: %w", plugin.Name, err)
	}

	// Overlay LSP server configurations from the plugin manifest.
	// Manifest-declared servers override same-named entries from the .lsp.json file.
	if plugin.Manifest.LspServers != nil {
		existingByName := make(map[string]int, len(configs))
		for i, cfg := range configs {
			existingByName[cfg.Name] = i
		}
		for name, manifestCfg := range plugin.Manifest.LspServers {
			if manifestCfg == nil {
				continue
			}
			entry := *manifestCfg
			entry.Name = name
			if idx, ok := existingByName[name]; ok {
				c := &configs[idx]
				if entry.Command != "" {
					(*c).Command = entry.Command
				}
				if len(entry.Args) > 0 {
					(*c).Args = entry.Args
				}
				if entry.Transport != "" {
					(*c).Transport = entry.Transport
				}
				if entry.Env != nil {
					if (*c).Env == nil {
						(*c).Env = make(map[string]string)
					}
					for k, v := range entry.Env {
						(*c).Env[k] = v
					}
				}
				if entry.InitializationOptions != nil {
					(*c).InitializationOptions = entry.InitializationOptions
				}
			} else {
				entry.PluginName = plugin.Name
				entry.PluginPath = plugin.Path
				entry.PluginSource = plugin.Name
				entry.Scope = "dynamic"
				configs = append(configs, &entry)
			}
		}
	}

	for _, cfg := range configs {
		cfg.PluginName = plugin.Name
		cfg.PluginPath = plugin.Path
		cfg.PluginSource = plugin.Name
		cfg.Scope = "dynamic"
	}

	logger.DebugCF("plugin.lsp_servers", "extracted LSP servers", map[string]any{
		"plugin": plugin.Name,
		"count":  len(configs),
	})
	return configs, nil
}

// LoadLspServersFromFile reads and parses an LSP server configuration JSON file.
// The file is expected to be a JSON object whose keys are server names and whose
// values are LspServerConfig objects. Each config's Name field is set from its
// map key.
//
// If the file does not exist the raw os.ErrNotExist is returned so that
// callers can distinguish the missing-file case with errors.Is. Other I/O and
// parse failures are returned as wrapped errors.
func LoadLspServersFromFile(filePath string) ([]*LspServerConfig, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		return nil, fmt.Errorf("failed to read LSP config %s: %w", filePath, err)
	}

	var raw map[string]*LspServerConfig
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse LSP config %s: %w", filePath, err)
	}

	configs := make([]*LspServerConfig, 0, len(raw))
	for name, cfg := range raw {
		cfg.Name = name
		configs = append(configs, cfg)
	}

	logger.DebugCF("plugin.lsp_servers", "loaded LSP servers from file", map[string]any{
		"file":  filePath,
		"count": len(configs),
	})
	return configs, nil
}

// ValidatePathWithinPlugin checks that a relative path stays within the plugin
// directory, preventing path traversal attacks.
//
// Both pluginPath and relativePath are resolved to absolute paths. The function
// then computes the relative path from the plugin root to the candidate. If the
// result starts with ".." or is itself absolute, the relativePath attempts to
// escape the plugin directory and the function returns an empty string.
//
// On success it returns the absolute, resolved path that is safely within the
// plugin directory.
func ValidatePathWithinPlugin(pluginPath, relativePath string) string {
	// Reject absolute paths outright — they cannot be relative to the plugin.
	if filepath.IsAbs(relativePath) {
		return ""
	}

	absPlugin, _ := filepath.Abs(pluginPath)
	candidate := filepath.Join(absPlugin, relativePath)
	absCandidate, _ := filepath.Abs(candidate)

	rel, _ := filepath.Rel(absPlugin, absCandidate)
	if strings.HasPrefix(rel, "..") {
		return ""
	}
	return absCandidate
}
