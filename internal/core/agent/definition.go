package agent

import (
	"github.com/sheepzhao/claude-code-go/internal/core/hook"
	"github.com/sheepzhao/claude-code-go/internal/core/tool"
)

// Definition holds the static configuration for an agent.
// It is the Go-side equivalent of the TypeScript AgentDefinition union type
// (BuiltInAgentDefinition | CustomAgentDefinition | PluginAgentDefinition).
//
// The Source field acts as the discriminator:
//   - "built-in"  → built-in agent
//   - "plugin"    → plugin-provided agent
//   - everything else ("userSettings", "projectSettings", etc.) → custom agent
type Definition struct {
	// AgentType is the stable identifier (e.g., "explore", "verify").
	AgentType string
	// WhenToUse describes when this agent should be invoked.
	WhenToUse string
	// Source identifies the origin: "built-in", "plugin", "userSettings", "projectSettings", etc.
	Source string
	// Tools is an allowlist of tool names. nil means no restriction.
	Tools []string
	// DisallowedTools is a denylist of tool names.
	DisallowedTools []string
	// Skills are skill names to preload.
	Skills []string
	// Model is the model override, or empty to inherit from parent.
	Model string
	// Effort is the effort level override.
	Effort string
	// PermissionMode is the permission mode override.
	PermissionMode string
	// MaxTurns is the maximum number of agentic turns before stopping.
	MaxTurns int
	// Background indicates the agent always runs as a background task.
	Background bool
	// Memory is the persistent memory scope: "user", "project", or "local".
	Memory string
	// Isolation is the isolation mode: "worktree" or "remote".
	Isolation string
	// OmitClaudeMd omits CLAUDE.md hierarchy from the agent's context.
	OmitClaudeMd bool
	// Plugin is the plugin name for plugin agents. Empty for non-plugin agents.
	Plugin string
	// Filename is the original filename without extension (for custom/plugin agents).
	Filename string
	// BaseDir is the directory where the agent definition was loaded from.
	BaseDir string
	// SystemPrompt stores a static system prompt for custom agents.
	// Built-in agents usually leave this empty and use SystemPromptProvider instead.
	SystemPrompt string
	// InitialPrompt is prepended to the first user turn.
	InitialPrompt string
	// CriticalSystemReminder is a short message re-injected at every user turn.
	CriticalSystemReminder string
	// SystemPromptProvider generates the system prompt for this agent.
	// Built-in agents use this to provide dynamic prompts; custom agents may leave it nil.
	SystemPromptProvider SystemPromptProvider

	// Color is the display color for this agent.
	// Valid values: "red", "blue", "green", "yellow", "purple", "orange", "pink", "cyan".
	// Empty means no custom color is set.
	Color string

	// MCPServers declares MCP servers specific to this agent.
	// Each entry is either a reference to an existing server by name, or an inline
	// server definition. Only used by custom agents; built-in agents leave this nil.
	MCPServers []AgentMCPServerSpec

	// Hooks declares session-scoped hooks registered when this agent starts.
	// Uses the same structure as global hooks configuration.
	// Only used by custom agents; built-in agents leave this nil.
	Hooks hook.HooksConfig
}

// AgentMCPServerSpec represents an MCP server declaration in an agent definition.
// It supports two forms:
//   1. A reference to an existing server by name (Name set, Config nil).
//   2. An inline server definition (Name set to the server name, Config holds
//      the raw configuration as a map).
//
// When Config is nil, the spec is a name-only reference.
type AgentMCPServerSpec struct {
	// Name is the server name. For references, this is the name of an existing
	// server configured in settings. For inline definitions, this is the key
	// from the frontmatter object.
	Name string
	// Config holds the raw server configuration for inline definitions.
	// When nil, this spec is a reference to an existing server by name.
	Config map[string]any
}

// SystemPromptProvider generates the system prompt for an agent.
// Built-in agents may use tool context; custom/plugin agents typically do not.
type SystemPromptProvider interface {
	// GetSystemPrompt returns the system prompt string for this agent.
	// The toolCtx parameter is used by built-in agents that need runtime context.
	GetSystemPrompt(toolCtx tool.UseContext) string
}

// IsBuiltIn reports whether the definition is a built-in agent.
func (d Definition) IsBuiltIn() bool {
	return d.Source == "built-in"
}

// IsPlugin reports whether the definition is a plugin agent.
func (d Definition) IsPlugin() bool {
	return d.Source == "plugin"
}

// IsCustom reports whether the definition is a user-defined or project-defined agent.
func (d Definition) IsCustom() bool {
	return !d.IsBuiltIn() && !d.IsPlugin()
}
