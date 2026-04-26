package builtin

import (
	"github.com/sheepzhao/claude-code-go/internal/core/agent"
	"github.com/sheepzhao/claude-code-go/internal/core/featureflag"
)

// RegisterBuiltInAgents registers all built-in agent definitions into the given registry.
func RegisterBuiltInAgents(registry agent.Registry) error {
	// Helper to register a built-in agent from its definition.
	register := func(def agent.BuiltInAgentDefinition) error {
		d := def.Definition
		d.SystemPromptProvider = def.SystemPromptProvider
		return registry.Register(d)
	}

	if featureflag.IsEnabled(featureflag.FlagBuiltinExplorePlanAgents) {
		if err := register(ExploreAgentDefinition); err != nil {
			return err
		}
		if err := register(PlanAgentDefinition); err != nil {
			return err
		}
	}

	if err := register(GeneralPurposeAgentDefinition); err != nil {
		return err
	}

	if featureflag.IsEnabled(featureflag.FlagVerificationAgent) {
		if err := register(VerificationAgentDefinition); err != nil {
			return err
		}
	}

	if err := register(StatuslineSetupAgentDefinition); err != nil {
		return err
	}
	if err := register(ClaudeCodeGuideAgentDefinition); err != nil {
		return err
	}

	return nil
}
