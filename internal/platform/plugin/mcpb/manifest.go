package mcpb

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// ParseManifest reads and parses a manifest.json file from a directory.
func ParseManifest(dir string) (*McpbManifest, error) {
	manifestPath := filepath.Join(dir, "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("manifest.json not found in %s", dir)
		}
		return nil, fmt.Errorf("failed to read manifest %s: %w", manifestPath, err)
	}

	return ParseManifestFromBytes(data)
}

// ParseManifestFromBytes parses a DXT manifest from raw JSON bytes and
// performs basic validation.
func ParseManifestFromBytes(data []byte) (*McpbManifest, error) {
	var manifest McpbManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest JSON: %w", err)
	}

	if err := validateManifest(&manifest); err != nil {
		return nil, err
	}

	logger.DebugCF("plugin.mcpb", "parsed manifest", map[string]any{
		"name":    manifest.Name,
		"version": manifest.Version,
	})

	return &manifest, nil
}

// validateManifest checks that required manifest fields are present.
func validateManifest(m *McpbManifest) error {
	if m.Name == "" {
		return fmt.Errorf("manifest is missing required field: name")
	}
	if m.Server == nil {
		return fmt.Errorf("manifest %q does not define a server configuration", m.Name)
	}
	if m.Server.Command == "" && m.Server.URL == "" {
		return fmt.Errorf("manifest %q server configuration must have command or url", m.Name)
	}
	return nil
}

// BuildMcpServerConfig constructs a McpbServerConfig from a parsed DXT
// manifest. The userConfig values are substituted into the server command
// arguments and environment variables where ${user_config.KEY} placeholders
// appear.
func BuildMcpServerConfig(manifest *McpbManifest, extractedPath string, userConfig UserConfigValues) (*McpbServerConfig, error) {
	if manifest.Server == nil {
		return nil, fmt.Errorf("manifest %q has no server definition", manifest.Name)
	}

	s := manifest.Server

	config := &McpbServerConfig{
		Name:      manifest.Name,
		Transport: s.Transport,
		Command:   s.Command,
		Args:      make([]string, len(s.Args)),
		Env:       make(map[string]string),
		URL:       s.URL,
		Headers:   make(map[string]string),
	}

	copy(config.Args, s.Args)
	for k, v := range s.Env {
		config.Env[k] = v
	}
	for k, v := range s.Headers {
		config.Headers[k] = v
	}

	// Default to stdio transport if not specified.
	if config.Transport == "" {
		config.Transport = "stdio"
	}

	// Resolve relative command paths against the extraction directory so
	// bundled binaries/scripts start from the correct working directory.
	if config.Command != "" && !filepath.IsAbs(config.Command) {
		config.Command = filepath.Join(extractedPath, config.Command)
	}

	// Apply user config substitution.
	if userConfig != nil {
		substituteUserConfig(config, userConfig)
	}

	logger.DebugCF("plugin.mcpb", "built MCP server config", map[string]any{
		"name":      config.Name,
		"transport": config.Transport,
		"command":   config.Command,
	})

	return config, nil
}

// substituteUserConfig replaces ${user_config.KEY} placeholders in command
// args and environment variable values with actual user configuration values.
func substituteUserConfig(config *McpbServerConfig, userConfig UserConfigValues) {
	for i, arg := range config.Args {
		config.Args[i] = replaceUserConfigPlaceholders(arg, userConfig)
	}
	for k, v := range config.Env {
		config.Env[k] = replaceUserConfigPlaceholders(v, userConfig)
	}
}

// replaceUserConfigPlaceholders finds ${user_config.KEY} patterns in s and
// replaces them with values from userConfig.
func replaceUserConfigPlaceholders(s string, userConfig UserConfigValues) string {
	result := s
	for key, val := range userConfig {
		placeholder := fmt.Sprintf("${user_config.%s}", key)
		result = replaceAll(result, placeholder, fmt.Sprintf("%v", val))
	}
	return result
}

// replaceAll is a simple string replacement helper.
func replaceAll(s, old, new string) string {
	result := ""
	for {
		i := 0
		for i < len(s) {
			if i+len(old) <= len(s) && s[i:i+len(old)] == old {
				result += new
				i += len(old)
			} else {
				result += string(s[i])
				i++
			}
		}
		return result
	}
}
