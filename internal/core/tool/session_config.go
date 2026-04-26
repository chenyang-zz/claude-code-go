package tool

// SkillInfo describes one custom skill or plugin skill visible to the guide agent.
type SkillInfo struct {
	// Name is the skill identifier (e.g. the slash command name).
	Name string
	// Description summarizes what the skill does.
	Description string
}

// AgentInfo describes one custom agent visible to the guide agent.
type AgentInfo struct {
	// AgentType is the stable identifier for the agent.
	AgentType string
	// WhenToUse describes when this agent should be invoked.
	WhenToUse string
}

// SessionConfigSnapshot captures the minimal session configuration visible to guide agent prompts.
// It mirrors the TypeScript toolUseContext.options subset used by claudeCodeGuideAgent.ts.
type SessionConfigSnapshot struct {
	// CustomSkills lists user-defined slash commands of type "prompt".
	CustomSkills []SkillInfo
	// CustomAgents lists user-defined agents (source != "built-in").
	CustomAgents []AgentInfo
	// MCPServers lists names of configured MCP servers.
	MCPServers []string
	// PluginSkills lists plugin-provided slash commands.
	PluginSkills []SkillInfo
	// UserSettings holds the filtered user settings object.
	UserSettings map[string]any
}

// IsEmpty reports whether the snapshot contains no visible configuration.
func (s SessionConfigSnapshot) IsEmpty() bool {
	return len(s.CustomSkills) == 0 &&
		len(s.CustomAgents) == 0 &&
		len(s.MCPServers) == 0 &&
		len(s.PluginSkills) == 0 &&
		len(s.UserSettings) == 0
}
