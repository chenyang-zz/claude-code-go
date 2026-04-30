package plugin

import (
	"path/filepath"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/agent"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// AgentRegistrar converts plugin AgentDefinition values into core agent.Definition
// values and registers them with an agent.Registry.
type AgentRegistrar struct {
	registry agent.Registry
}

// NewAgentRegistrar creates an agent registrar backed by the given registry.
func NewAgentRegistrar(registry agent.Registry) *AgentRegistrar {
	return &AgentRegistrar{registry: registry}
}

// RegisterAgents converts and registers every agent definition from the given
// slice.  Duplicate agent types are handled by removing the existing entry
// first (matching TypeScript getActiveAgentsFromList priority where later
// sources override earlier ones).  Registration errors are collected and
// returned alongside the count of successfully registered agents.
func (r *AgentRegistrar) RegisterAgents(defs []*AgentDefinition) (registered int, errs []*PluginError) {
	if r == nil || r.registry == nil {
		return 0, []*PluginError{{
			Type:    "registration-error",
			Source:  "agent-registrar",
			Message: "agent registrar or registry is nil",
		}}
	}

	for _, def := range defs {
		if def == nil {
			continue
		}

		coreDef := toCoreAgentDefinition(def)

		// Remove existing agent of the same type before registering,
		// matching TypeScript override semantics.
		r.registry.Remove(coreDef.AgentType)

		if err := r.registry.Register(coreDef); err != nil {
			errs = append(errs, &PluginError{
				Type:    "agent-registration-error",
				Source:  "agent-registrar",
				Plugin:  def.PluginName,
				Message: err.Error(),
			})
			logger.WarnCF("plugin.agent_registrar", "failed to register agent", map[string]any{
				"agent_type":  coreDef.AgentType,
				"plugin":      def.PluginName,
				"error":       err.Error(),
			})
			continue
		}

		registered++
		logger.InfoCF("plugin.agent_registrar", "registered agent", map[string]any{
			"agent_type": coreDef.AgentType,
			"plugin":     def.PluginName,
		})
	}

	return registered, errs
}

// toCoreAgentDefinition maps a plugin AgentDefinition to the core
// agent.Definition used by the runtime.
func toCoreAgentDefinition(def *AgentDefinition) agent.Definition {
	whenToUse := def.WhenToUse
	if whenToUse == "" {
		whenToUse = def.Description
	}

	d := agent.Definition{
		AgentType:       def.AgentType,
		WhenToUse:       whenToUse,
		Source:          "plugin",
		Tools:           parseCommaSeparated(def.Tools),
		DisallowedTools: parseCommaSeparated(def.DisallowedTools),
		Skills:          parseCommaSeparated(def.Skills),
		Model:           def.Model,
		Effort:          def.Effort,
		MaxTurns:        def.MaxTurns,
		Background:      def.Background,
		Memory:          def.Memory,
		Isolation:       def.Isolation,
		Plugin:          def.PluginName,
		SystemPrompt:    def.RawContent,
		Color:           def.Color,
	}

	if def.SourcePath != "" {
		d.Filename = filepath.Base(def.SourcePath)
		ext := filepath.Ext(d.Filename)
		if ext != "" {
			d.Filename = d.Filename[:len(d.Filename)-len(ext)]
		}
		d.BaseDir = filepath.Dir(def.SourcePath)
	}

	return d
}

// parseCommaSeparated splits a comma-separated string into a slice of trimmed
// non-empty strings.  The special value "*" or an empty string returns nil,
// indicating no restriction.
func parseCommaSeparated(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" || s == "*" {
		return nil
	}

	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}
