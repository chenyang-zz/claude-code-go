package plugin

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sheepzhao/claude-code-go/internal/platform/plugin/mcpb"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// ExtractMcpServers extracts MCP server configurations from a plugin. It
// reads from the plugin's .mcp.json file and then overlays settings from
// manifest.mcpServers. The manifest overlay is deferred until PluginManifest
// gains an mcpServers field.
func ExtractMcpServers(plugin *LoadedPlugin) ([]*McpServerConfig, error) {
	mcpPath := filepath.Join(plugin.Path, ".mcp.json")
	configs, err := LoadMcpServersFromFile(mcpPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to load MCP servers for plugin %s: %w", plugin.Name, err)
	}

	// Overlay MCP server configurations from the plugin manifest.
	// Manifest-declared servers override same-named entries from the .mcp.json file.
	if plugin.Manifest.McpServers != nil {
		existingByName := make(map[string]int, len(configs))
		for i, cfg := range configs {
			existingByName[cfg.Name] = i
		}
		for name, manifestCfg := range plugin.Manifest.McpServers {
			if manifestCfg == nil {
				continue
			}
			entry := *manifestCfg
			entry.Name = name
			if idx, ok := existingByName[name]; ok {
				c := &configs[idx]
				if entry.Transport != "" {
					(*c).Transport = entry.Transport
				}
				if entry.Command != "" {
					(*c).Command = entry.Command
				}
				if len(entry.Args) > 0 {
					(*c).Args = entry.Args
				}
				if entry.URL != "" {
					(*c).URL = entry.URL
				}
				if len(entry.Headers) > 0 {
					(*c).Headers = entry.Headers
				}
				if entry.Env != nil {
					if (*c).Env == nil {
						(*c).Env = make(map[string]string)
					}
					for k, v := range entry.Env {
						(*c).Env[k] = v
					}
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

	logger.DebugCF("plugin.mcp_servers", "extracted MCP servers", map[string]any{
		"plugin": plugin.Name,
		"count":  len(configs),
	})
	return configs, nil
}

// LoadMcpServersFromFile reads and parses an MCP server configuration JSON
// file. The file is expected to be in one of two formats:
//
//  1. Wrapper format: {"mcpServers": {"serverName": {...}, ...}}
//  2. Direct format:  {"serverName": {...}, ...}
//
// Each config's Name field is set from its map key. If the file does not
// exist, the raw os.ErrNotExist is returned so callers can distinguish the
// missing-file case with errors.Is.
func LoadMcpServersFromFile(filePath string) ([]*McpServerConfig, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		return nil, fmt.Errorf("failed to read MCP config %s: %w", filePath, err)
	}

	// Parse into raw map to handle both wrapper and direct formats.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse MCP config %s: %w", filePath, err)
	}

	// Check for "mcpServers" wrapper key and unwrap if present.
	if serversRaw, ok := raw["mcpServers"]; ok {
		var servers map[string]json.RawMessage
		if err := json.Unmarshal(serversRaw, &servers); err == nil {
			raw = servers
		}
	}

	var configs []*McpServerConfig
	for name, rawCfg := range raw {
		// Skip known non-server keys that might appear at the top level.
		if name == "mcpServers" {
			continue
		}
		var cfg McpServerConfig
		if err := json.Unmarshal(rawCfg, &cfg); err != nil {
			logger.DebugCF("plugin.mcp_servers", "skipping invalid MCP server entry", map[string]any{
				"file":   filePath,
				"server": name,
				"error":  err.Error(),
			})
			continue
		}
		cfg.Name = name
		configs = append(configs, &cfg)
	}

	logger.DebugCF("plugin.mcp_servers", "loaded MCP servers from file", map[string]any{
		"file":  filePath,
		"count": len(configs),
	})
	return configs, nil
}

// ExtractMcpbServers discovers and loads MCPB (.mcpb / .dxt) files from a
// plugin directory. For each MCPB source found, it loads the packaged MCP
// server configuration and returns it as a McpServerConfig with IsMcpb set
// to true. MCPB loading failures are non-fatal — the function returns as many
// configs as it can successfully load and logs errors for the rest.
func ExtractMcpbServers(plugin *LoadedPlugin) ([]*McpServerConfig, error) {
	sources := discoverMcpbSources(plugin.Path)
	if len(sources) == 0 {
		return nil, nil
	}

	var configs []*McpServerConfig
	for _, src := range sources {
		cl := &mcpb.ConfigLoader{
			PluginPath: plugin.Path,
			PluginID:   plugin.Name,
		}
		result, needsConfig, err := mcpb.LoadMcpbConfig(src, cl, nil, nil)
		if err != nil {
			logger.DebugCF("plugin.mcpb", "failed to load MCPB config", map[string]any{
				"source": src,
				"plugin": plugin.Name,
				"error":  err.Error(),
			})
			continue
		}
		if needsConfig != nil {
			logger.WarnCF("plugin.mcpb", "MCPB server requires user configuration", map[string]any{
				"source":     src,
				"plugin":     plugin.Name,
				"server":     needsConfig.Manifest.Name,
				"fieldCount": len(needsConfig.ConfigSchema),
			})
			continue
		}
		if result == nil {
			continue
		}

		cfg := &McpServerConfig{
			Name:         result.McpConfig.Name,
			Transport:    result.McpConfig.Transport,
			Command:      result.McpConfig.Command,
			Args:         result.McpConfig.Args,
			Env:          result.McpConfig.Env,
			URL:          result.McpConfig.URL,
			Headers:      result.McpConfig.Headers,
			PluginName:   plugin.Name,
			PluginPath:   plugin.Path,
			PluginSource: plugin.Name,
			Scope:        "dynamic",
			IsMcpb:       true,
		}
		configs = append(configs, cfg)
	}

	logger.DebugCF("plugin.mcpb", "extracted MCPB servers", map[string]any{
		"plugin": plugin.Name,
		"total":  len(sources),
		"loaded": len(configs),
	})
	return configs, nil
}

// discoverMcpbSources finds .mcpb and .dxt files directly inside the plugin
// root directory. It returns relative paths suitable for passing to the MCPB
// loader (which resolves them relative to the plugin path).
func discoverMcpbSources(pluginPath string) []string {
	entries, err := os.ReadDir(pluginPath)
	if err != nil {
		return nil
	}

	var sources []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if mcpb.IsMcpbSource(name) {
			sources = append(sources, name)
		}
	}
	return sources
}
