// Package mcpb provides MCPB (.mcpb / .dxt) file handling for the plugin system.
// It supports downloading, extracting, manifest parsing, MCP server config
// generation, user configuration management, and caching for packaged MCP
// server bundles.
package mcpb

// ProgressCallback is called during long-running operations to report status.
type ProgressCallback func(status string)

// UserConfigValues holds user-provided configuration values for an MCP server.
// Keys map to field names declared in the DXT manifest's user_config schema.
type UserConfigValues map[string]any

// McpbLoadResult is returned when an MCPB file is successfully loaded and the
// MCP server configuration has been generated.
type McpbLoadResult struct {
	// Manifest holds the parsed DXT manifest.
	Manifest McpbManifest
	// McpConfig is the generated MCP server configuration.
	McpConfig McpbServerConfig
	// ExtractedPath is the filesystem path where the MCPB contents were extracted.
	ExtractedPath string
	// ContentHash is the SHA-256 hash of the MCPB file data (first 16 hex chars).
	ContentHash string
}

// McpbNeedsConfigResult is returned when an MCPB file requires user
// configuration before the MCP server can be started.
type McpbNeedsConfigResult struct {
	// Status is always "needs-config" for this result type.
	Status string
	// Manifest holds the parsed DXT manifest.
	Manifest McpbManifest
	// ExtractedPath is the filesystem path where the MCPB contents were extracted.
	ExtractedPath string
	// ContentHash is the SHA-256 hash of the MCPB file content.
	ContentHash string
	// ConfigSchema describes the user configuration options required by the
	// manifest. Keys are field names, values are option descriptors.
	ConfigSchema map[string]McpbConfigOption
	// ExistingConfig holds any previously saved configuration values.
	ExistingConfig UserConfigValues
	// ValidationErrors lists validation failures for the current configuration.
	ValidationErrors []string
}

// McpbCacheMetadata stores information about a cached MCPB extraction.
type McpbCacheMetadata struct {
	// Source is the original MCPB file path or URL.
	Source string `json:"source"`
	// ContentHash is the SHA-256 hash of the MCPB file content.
	ContentHash string `json:"contentHash"`
	// ExtractedPath is where the MCPB contents were extracted on disk.
	ExtractedPath string `json:"extractedPath"`
	// CachedAt is the ISO 8601 timestamp when the cache entry was created.
	CachedAt string `json:"cachedAt"`
	// LastChecked is the ISO 8601 timestamp of the most recent cache check.
	LastChecked string `json:"lastChecked"`
}

// McpbManifest is a minimal representation of a DXT manifest. It only includes
// the fields needed to build an MCP server configuration.
type McpbManifest struct {
	// Name is the server or package identifier.
	Name string `json:"name"`
	// Version is a semantic version string.
	Version string `json:"version,omitempty"`
	// Author describes the package author.
	Author McpbManifestAuthor `json:"author,omitempty"`
	// Server holds the MCP server definition.
	Server *McpbManifestServer `json:"server,omitempty"`
	// UserConfig declares user-configurable options for the server.
	UserConfig map[string]McpbConfigOption `json:"user_config,omitempty"`
}

// McpbManifestAuthor holds the author information from a DXT manifest.
type McpbManifestAuthor struct {
	// Name is the author or organization name.
	Name string `json:"name"`
	// Email is an optional contact email.
	Email string `json:"email,omitempty"`
	// URL is an optional homepage or profile URL.
	URL string `json:"url,omitempty"`
}

// McpbManifestServer defines the MCP server process in a DXT manifest.
type McpbManifestServer struct {
	// Command is the executable to launch.
	Command string `json:"command"`
	// Args are the command-line arguments.
	Args []string `json:"args,omitempty"`
	// Env holds environment variables for the server process.
	Env map[string]string `json:"env,omitempty"`
	// Transport is the MCP transport type (e.g. "stdio", "sse").
	Transport string `json:"transport,omitempty"`
	// URL is the endpoint for remote transports.
	URL string `json:"url,omitempty"`
	// Headers are HTTP headers for remote transports.
	Headers map[string]string `json:"headers,omitempty"`
}

// McpbConfigOption describes a single user-configurable field in a DXT
// manifest's user_config section.
type McpbConfigOption struct {
	// Type is the value type: "string", "number", "boolean", "file", or
	// "directory".
	Type string `json:"type"`
	// Required indicates the field must be filled in.
	Required bool `json:"required,omitempty"`
	// Title is a short human-readable label for the field.
	Title string `json:"title,omitempty"`
	// Description explains the field's purpose.
	Description string `json:"description,omitempty"`
	// Sensitive indicates the value should be stored in secure storage rather
	// than plain text settings.
	Sensitive bool `json:"sensitive,omitempty"`
	// Default is the default value when the user has not provided one.
	Default any `json:"default,omitempty"`
	// Min is an optional minimum value for number types.
	Min *float64 `json:"min,omitempty"`
	// Max is an optional maximum value for number types.
	Max *float64 `json:"max,omitempty"`
	// Multiple indicates the value can be an array of the declared type.
	Multiple bool `json:"multiple,omitempty"`
}

// McpbServerConfig is the MCP server configuration produced from an MCPB
// manifest. It mirrors the fields needed to register an MCP server.
type McpbServerConfig struct {
	// Name is the server identifier.
	Name string `json:"name"`
	// Transport is the MCP transport type.
	Transport string `json:"transport,omitempty"`
	// Command is the executable path for stdio transport.
	Command string `json:"command,omitempty"`
	// Args are the command arguments.
	Args []string `json:"args,omitempty"`
	// Env holds environment variables for the server process.
	Env map[string]string `json:"env,omitempty"`
	// URL is the endpoint for remote transports.
	URL string `json:"url,omitempty"`
	// Headers are HTTP headers for remote transports.
	Headers map[string]string `json:"headers,omitempty"`
}
