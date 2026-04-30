// Package plugin provides the plugin system foundation for Claude Code.
// It handles plugin type definitions, directory discovery, manifest loading,
// and registration. Marketplace, installation, and capability-specific loading
// are handled by other packages or future batches.
package plugin

import "strings"

// PluginSourceType identifies the type of source a plugin originates from.
type PluginSourceType string

const (
	// SourceTypePath indicates a plugin loaded from a local filesystem path.
	SourceTypePath PluginSourceType = "path"
	// SourceTypeGit indicates a plugin sourced from a Git repository.
	SourceTypeGit PluginSourceType = "git"
	// SourceTypeGitHub indicates a plugin sourced from a GitHub repository.
	SourceTypeGitHub PluginSourceType = "github"
	// SourceTypeNPM indicates a plugin sourced from an npm package.
	SourceTypeNPM PluginSourceType = "npm"
	// SourceTypeBuiltin indicates a plugin that ships with the CLI.
	SourceTypeBuiltin PluginSourceType = "builtin"
)

// PluginSource describes the origin of a plugin.
type PluginSource struct {
	// Type is the kind of source (path, git, github, npm, builtin).
	Type PluginSourceType `json:"type"`
	// Value is the source identifier (e.g. filesystem path, repo URL, package name).
	Value string `json:"value"`
	// Version is an optional version constraint or ref.
	Version string `json:"version,omitempty"`
}

// PluginAuthor holds information about the plugin creator or maintainer.
type PluginAuthor struct {
	// Name is the display name of the author or organization.
	Name string `json:"name"`
	// Email is an optional contact email address.
	Email string `json:"email,omitempty"`
	// URL is an optional homepage or profile URL.
	URL string `json:"url,omitempty"`
}

// PluginManifest holds the metadata for a plugin as defined in its manifest file
// (plugin.json or package.json). Only metadata fields are included; capability
// definitions (hooks, commands, agents, MCP servers, LSP servers) are loaded
// separately by their respective subsystems.
type PluginManifest struct {
	// Name is the unique plugin identifier. Must not contain spaces.
	Name string `json:"name"`
	// Version is an optional semantic version string.
	Version string `json:"version,omitempty"`
	// Description is a brief, user-facing explanation of what the plugin provides.
	Description string `json:"description,omitempty"`
	// Author holds information about the plugin creator.
	Author *PluginAuthor `json:"author,omitempty"`
	// Homepage is an optional URL for the plugin's homepage or documentation.
	Homepage string `json:"homepage,omitempty"`
	// Repository is an optional URL for the plugin's source code repository.
	Repository string `json:"repository,omitempty"`
	// License is an optional SPDX license identifier.
	License string `json:"license,omitempty"`
	// Keywords are optional tags for plugin discovery and categorization.
	Keywords []string `json:"keywords,omitempty"`
}

// LoadedPlugin represents a plugin that has been discovered and loaded into
// the registry. It combines the manifest metadata with runtime state such as
// the filesystem path and enable/disable status, plus auto-detected capability
// directory paths.
type LoadedPlugin struct {
	// Name is the plugin identifier (from the manifest).
	Name string `json:"name"`
	// Manifest holds the parsed plugin metadata.
	Manifest PluginManifest `json:"manifest"`
	// Path is the absolute filesystem path to the plugin root directory.
	Path string `json:"path"`
	// Source describes where the plugin was loaded from.
	Source PluginSource `json:"source"`
	// Enabled indicates whether the plugin is currently active.
	Enabled bool `json:"enabled"`
	// IsBuiltin is true for plugins that ship with the CLI.
	IsBuiltin bool `json:"isBuiltin"`
	// CommandsPath is the path to the auto-detected commands/ subdirectory.
	CommandsPath string `json:"commandsPath,omitempty"`
	// SkillsPath is the path to the auto-detected skills/ subdirectory.
	SkillsPath string `json:"skillsPath,omitempty"`
	// AgentsPath is the path to the auto-detected agents/ subdirectory.
	AgentsPath string `json:"agentsPath,omitempty"`
	// OutputStylesPath is the path to the auto-detected output-styles/ subdirectory.
	OutputStylesPath string `json:"outputStylesPath,omitempty"`
	// HooksConfig is the loaded hooks configuration from hooks/hooks.json.
	HooksConfig *HooksConfig `json:"hooksConfig,omitempty"`
}

// HooksConfig represents the loaded hooks configuration for a plugin.
// It maps hook event names to their matcher entries.
type HooksConfig struct {
	// Events maps hook event names (e.g. "PreToolUse", "Stop") to their
	// matcher entries.
	Events map[string][]HookMatcherEntry `json:"events"`
}

// HookMatcherEntry represents a single matcher entry in hooks configuration.
type HookMatcherEntry struct {
	// Matcher is an optional regex or exact string to match against tool
	// names or other identifiers.
	Matcher string `json:"matcher,omitempty"`
	// Hooks is the list of hook commands or HTTP calls to execute when the
	// matcher matches.
	Hooks []HookCommand `json:"hooks"`
}

// HookCommand represents a single hook command or HTTP call configuration.
type HookCommand struct {
	// Type is the hook type ("command" or "http").
	Type string `json:"type"`
	// Command is the shell command to execute (for type "command").
	Command string `json:"command,omitempty"`
	// Timeout is an optional timeout in milliseconds.
	Timeout int `json:"timeout,omitempty"`
}

// PluginHookMatcher enriches HookMatcherEntry with plugin context information.
// This is the type that gets registered into the hook system.
type PluginHookMatcher struct {
	// Matcher is an optional regex or exact string to match against.
	Matcher string `json:"matcher,omitempty"`
	// Hooks is the list of hook commands to execute.
	Hooks []HookCommand `json:"hooks"`
	// PluginRoot is the absolute path to the plugin's root directory.
	PluginRoot string `json:"pluginRoot"`
	// PluginName is the human-readable plugin name.
	PluginName string `json:"pluginName"`
	// PluginID is the plugin identifier in "name@source" format.
	PluginID string `json:"pluginId"`
}

// PluginCommand represents a command (or skill) extracted from a plugin's
// commands/ or skills/ directory.
type PluginCommand struct {
	// Name is the fully namespaced command name (e.g. "pluginName:commandName").
	Name string `json:"name"`
	// DisplayName is a human-readable name from frontmatter, falling back to Name.
	DisplayName string `json:"displayName,omitempty"`
	// Description is the command description from frontmatter or markdown body.
	Description string `json:"description"`
	// PluginName is the originating plugin's name.
	PluginName string `json:"pluginName"`
	// PluginPath is the absolute path to the plugin root directory.
	PluginPath string `json:"pluginPath"`
	// SourcePath is the absolute path to the source .md file.
	SourcePath string `json:"sourcePath"`
	// IsSkill is true if this command was loaded as a skill.
	IsSkill bool `json:"isSkill"`
	// RawContent is the markdown body after the YAML frontmatter.
	RawContent string `json:"rawContent"`
	// AllowedTools is the allowed-tools frontmatter field.
	AllowedTools string `json:"allowedTools,omitempty"`
	// ArgumentHint is the argument-hint frontmatter field.
	ArgumentHint string `json:"argumentHint,omitempty"`
	// ArgumentNames is the list of argument names from the frontmatter arguments
	// field, used for named parameter substitution ($name).
	ArgumentNames []string `json:"argumentNames,omitempty"`
	// PluginSource is the plugin source identifier used for resolving the
	// ${CLAUDE_PLUGIN_DATA} variable.
	PluginSource string `json:"pluginSource,omitempty"`
	// WhenToUse is the when_to_use frontmatter field.
	WhenToUse string `json:"whenToUse,omitempty"`
	// Version is the version frontmatter field.
	Version string `json:"version,omitempty"`
	// Model is the model frontmatter field.
	Model string `json:"model,omitempty"`
	// Effort is the effort frontmatter field.
	Effort string `json:"effort,omitempty"`
	// UserInvocable is the user-invocable frontmatter field (defaults to true).
	UserInvocable bool `json:"userInvocable"`
	// DisableModelInvocation is the disable-model-invocation frontmatter field.
	DisableModelInvocation bool `json:"disableModelInvocation"`
	// Shell is the shell frontmatter field (defaults to "bash").
	Shell string `json:"shell,omitempty"`
}

// OutputStyleConfig represents an output style extracted from a plugin's
// output-styles/ directory.
type OutputStyleConfig struct {
	// Name is the namespaced style name (e.g. "pluginName:styleName").
	Name string `json:"name"`
	// Description is the style description from frontmatter or first paragraph.
	Description string `json:"description"`
	// Prompt is the markdown body content (the style instructions).
	Prompt string `json:"prompt"`
	// ForceForPlugin indicates whether this style should be auto-applied
	// when the plugin is enabled.
	ForceForPlugin bool `json:"forceForPlugin"`
	// PluginName is the originating plugin's name.
	PluginName string `json:"pluginName"`
}

// PluginError represents an error encountered during plugin loading or
// operation. It carries contextual information for debugging and user guidance.
type PluginError struct {
	// Type is an error category key (e.g. "manifest-parse-error",
	// "manifest-validation-error", "path-not-found").
	Type string `json:"type"`
	// Source identifies where the error originated (e.g. plugin name or
	// marketplace name).
	Source string `json:"source"`
	// Plugin is the name of the plugin associated with the error, if any.
	Plugin string `json:"plugin,omitempty"`
	// Message is a human-readable description of the error.
	Message string `json:"message"`
}

// Error implements the error interface.
func (e *PluginError) Error() string {
	return e.Message
}

// ParsedAllowedTools parses the AllowedTools frontmatter field into a slice of
// individual tool names. Returns nil if AllowedTools is empty.
func (pc *PluginCommand) ParsedAllowedTools() []string {
	if pc == nil || pc.AllowedTools == "" {
		return nil
	}
	tools := strings.Split(pc.AllowedTools, ",")
	for i := range tools {
		tools[i] = strings.TrimSpace(tools[i])
	}
	return tools
}

// PluginLoadResult aggregates the results of a plugin loading operation.
type PluginLoadResult struct {
	// Enabled is the list of plugins that were loaded and are active.
	Enabled []*LoadedPlugin `json:"enabled"`
	// Disabled is the list of plugins that were loaded but are not active.
	Disabled []*LoadedPlugin `json:"disabled"`
	// Errors collects any errors encountered during loading.
	Errors []*PluginError `json:"errors,omitempty"`
}

// AgentDefinition represents an agent extracted from a plugin's agents/
// directory. It holds the frontmatter metadata and the markdown system prompt
// content.
type AgentDefinition struct {
	// AgentType is the fully namespaced agent type name (e.g. "pluginName:agentName").
	AgentType string `json:"agentType"`
	// DisplayName is a human-readable name from frontmatter, falling back to AgentType.
	DisplayName string `json:"displayName,omitempty"`
	// Description is the agent description from frontmatter.
	Description string `json:"description"`
	// WhenToUse describes when the agent should be invoked.
	WhenToUse string `json:"whenToUse,omitempty"`
	// PluginName is the originating plugin name.
	PluginName string `json:"pluginName"`
	// PluginPath is the absolute path to the plugin root directory.
	PluginPath string `json:"pluginPath"`
	// SourcePath is the absolute path to the source .md file.
	SourcePath string `json:"sourcePath"`
	// Tools is the tools frontmatter field ("*" means all, "" means unspecified).
	Tools string `json:"tools,omitempty"`
	// Skills is the skills frontmatter field.
	Skills string `json:"skills,omitempty"`
	// Color is the agent avatar color from frontmatter.
	Color string `json:"color,omitempty"`
	// Model is the model override from frontmatter ("inherit" means use default).
	Model string `json:"model,omitempty"`
	// Background indicates the agent should run as a background task.
	Background bool `json:"background"`
	// Memory is the memory scope (user, project, local).
	Memory string `json:"memory,omitempty"`
	// Isolation is the isolation mode (only "worktree" is valid).
	Isolation string `json:"isolation,omitempty"`
	// Effort is the effort level from frontmatter.
	Effort string `json:"effort,omitempty"`
	// MaxTurns is the maximum number of conversation turns.
	MaxTurns int `json:"maxTurns,omitempty"`
	// DisallowedTools is the list of tools the agent cannot use.
	DisallowedTools string `json:"disallowedTools,omitempty"`
	// RawContent is the markdown body (system prompt content).
	RawContent string `json:"rawContent"`
}

// McpServerConfig represents an MCP server configuration extracted from a
// plugin's .mcp.json or manifest.mcpServers field.
type McpServerConfig struct {
	// Name is the server name (used as the key in the servers map).
	Name string `json:"name"`
	// Transport is the transport type: stdio, sse, http, ws, sse-ide, ws-ide,
	// sdk, or claudeai-proxy.
	Transport string `json:"transport,omitempty"`
	// Command is the executable path (for stdio transport).
	Command string `json:"command,omitempty"`
	// Args are the command arguments (for stdio transport).
	Args []string `json:"args,omitempty"`
	// Env holds environment variables for the server process.
	Env map[string]string `json:"env,omitempty"`
	// URL is the endpoint URL (for sse, http, ws transports).
	URL string `json:"url,omitempty"`
	// Headers are HTTP headers (for sse, http transports).
	Headers map[string]string `json:"headers,omitempty"`
	// PluginName is the originating plugin name.
	PluginName string `json:"pluginName,omitempty"`
	// PluginPath is the absolute path to the plugin root directory.
	PluginPath string `json:"pluginPath,omitempty"`
	// PluginSource is the plugin source identifier used for data directory resolution.
	PluginSource string `json:"pluginSource,omitempty"`
	// Scope identifies this as a plugin-scoped server ("dynamic").
	Scope string `json:"scope,omitempty"`
	// IsMcpb indicates this was loaded from an MCPB source.
	IsMcpb bool `json:"isMcpb,omitempty"`
}

// LspServerConfig represents an LSP server configuration extracted from a
// plugin's .lsp.json or manifest.lspServers field.
type LspServerConfig struct {
	// Name is the server name (used as the key in the servers map).
	Name string `json:"name"`
	// Command is the LSP server executable path.
	Command string `json:"command"`
	// Args are the command arguments.
	Args []string `json:"args,omitempty"`
	// ExtensionToLanguage maps file extensions to LSP language IDs.
	ExtensionToLanguage map[string]string `json:"extensionToLanguage"`
	// Transport is the transport type: "stdio" (default) or "socket".
	Transport string `json:"transport,omitempty"`
	// Env holds environment variables for the server process.
	Env map[string]string `json:"env,omitempty"`
	// InitializationOptions are passed during the initialize request.
	InitializationOptions map[string]any `json:"initializationOptions,omitempty"`
	// Settings are passed via workspace/didChangeConfiguration.
	Settings map[string]any `json:"settings,omitempty"`
	// WorkspaceFolder is the workspace folder path.
	WorkspaceFolder string `json:"workspaceFolder,omitempty"`
	// StartupTimeout is the timeout in ms for startup.
	StartupTimeout int `json:"startupTimeout,omitempty"`
	// ShutdownTimeout is the timeout in ms for graceful shutdown.
	ShutdownTimeout int `json:"shutdownTimeout,omitempty"`
	// RestartOnCrash enables auto-restart on crash.
	RestartOnCrash bool `json:"restartOnCrash"`
	// MaxRestarts is the maximum restart attempts.
	MaxRestarts int `json:"maxRestarts,omitempty"`
	// PluginName is the originating plugin name.
	PluginName string `json:"pluginName,omitempty"`
	// PluginPath is the absolute path to the plugin root directory.
	PluginPath string `json:"pluginPath,omitempty"`
	// PluginSource is the plugin source identifier used for data directory resolution.
	PluginSource string `json:"pluginSource,omitempty"`
	// Scope identifies this as a plugin-scoped server ("dynamic").
	Scope string `json:"scope,omitempty"`
}

// RefreshResult holds the counts from a plugin refresh operation.
type RefreshResult struct {
	// EnabledCount is the number of enabled plugins after refresh.
	EnabledCount int `json:"enabledCount"`
	// DisabledCount is the number of disabled plugins after refresh.
	DisabledCount int `json:"disabledCount"`
	// CommandCount is the number of plugin commands loaded.
	CommandCount int `json:"commandCount"`
	// AgentCount is the number of plugin agents loaded.
	AgentCount int `json:"agentCount"`
	// McpCount is the number of plugin MCP servers loaded.
	McpCount int `json:"mcpCount"`
	// LspCount is the number of plugin LSP servers loaded.
	LspCount int `json:"lspCount"`
	// HookCount is the number of plugin hook events loaded.
	HookCount int `json:"hookCount"`
	// ErrorCount is the number of errors encountered during refresh.
	ErrorCount int `json:"errorCount"`
	// Commands is the list of loaded plugin commands.
	Commands []*PluginCommand `json:"commands,omitempty"`
	// Agents is the list of loaded plugin agents.
	Agents []*AgentDefinition `json:"agents,omitempty"`
	// Plugins is the list of enabled plugins after refresh.  Preserved so
	// that downstream registrars can extract hooks, MCP servers, and LSP
	// servers without re-loading from disk.
	Plugins []*LoadedPlugin `json:"-"`
}
