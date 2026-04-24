package agent

// BuiltInAgentDefinition wraps a standard Definition with built-in-specific metadata.
type BuiltInAgentDefinition struct {
	Definition
	// SystemPromptProvider generates the system prompt for this built-in agent.
	// If nil, the agent uses a static system prompt or the parent's default.
	SystemPromptProvider SystemPromptProvider
}

// NewBuiltInAgentDefinition creates a BuiltInAgentDefinition with Source and BaseDir
// pre-set to built-in values.
func NewBuiltInAgentDefinition(agentType string, provider SystemPromptProvider) BuiltInAgentDefinition {
	return BuiltInAgentDefinition{
		Definition: Definition{
			AgentType: agentType,
			Source:    "built-in",
			BaseDir:   "built-in",
		},
		SystemPromptProvider: provider,
	}
}
