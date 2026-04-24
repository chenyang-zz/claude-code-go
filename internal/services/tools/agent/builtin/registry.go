package builtin

import (
	"github.com/sheepzhao/claude-code-go/internal/core/agent"
)

// RegisterBuiltInAgents registers all built-in agent definitions into the given registry.
func RegisterBuiltInAgents(registry agent.Registry) error {
	exploreDef := ExploreAgentDefinition.Definition
	exploreDef.SystemPromptProvider = ExploreAgentDefinition.SystemPromptProvider
	if err := registry.Register(exploreDef); err != nil {
		return err
	}
	return nil
}
