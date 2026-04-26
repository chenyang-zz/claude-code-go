package loader

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/agent"
)

// ParseAgentFromJson parses a single agent definition from JSON data.
// The definition parameter should be a JSON object.
// Returns an error if required fields (description, prompt) are missing.
func ParseAgentFromJson(name string, definition json.RawMessage, source string) (agent.Definition, error) {
	if source == "" {
		source = "flagSettings"
	}

	// Unmarshal into a map so we can handle fields individually
	// and silently ignore unknown fields.
	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(definition, &rawMap); err != nil {
		return agent.Definition{}, fmt.Errorf("invalid JSON object for agent %q: %w", name, err)
	}

	// Required fields.
	var description string
	if raw, ok := rawMap["description"]; ok {
		if err := json.Unmarshal(raw, &description); err != nil {
			return agent.Definition{}, fmt.Errorf("agent %q: invalid 'description' field: %w", name, err)
		}
		description = strings.TrimSpace(description)
	}
	if description == "" {
		return agent.Definition{}, fmt.Errorf("agent %q: missing required 'description' field", name)
	}

	var prompt string
	if raw, ok := rawMap["prompt"]; ok {
		if err := json.Unmarshal(raw, &prompt); err != nil {
			return agent.Definition{}, fmt.Errorf("agent %q: invalid 'prompt' field: %w", name, err)
		}
		prompt = strings.TrimSpace(prompt)
	}
	if prompt == "" {
		return agent.Definition{}, fmt.Errorf("agent %q: missing required 'prompt' field", name)
	}

	def := agent.Definition{
		AgentType:    name,
		WhenToUse:    description,
		Source:       source,
		SystemPrompt: prompt,
	}

	// Tools: nil means all tools, [] means no tools, ["*"] means all tools.
	if raw, ok := rawMap["tools"]; ok {
		var tools []string
		if err := json.Unmarshal(raw, &tools); err == nil {
			def.Tools = parseToolsFromStrings(tools)
		}
	}

	// DisallowedTools: same parsing semantics as tools.
	if raw, ok := rawMap["disallowedTools"]; ok {
		var disallowed []string
		if err := json.Unmarshal(raw, &disallowed); err == nil {
			def.DisallowedTools = parseToolsFromStrings(disallowed)
		}
	}

	// Skills: string array.
	if raw, ok := rawMap["skills"]; ok {
		var skills []string
		if err := json.Unmarshal(raw, &skills); err == nil {
			def.Skills = skills
		}
	}

	// Model: trim whitespace; preserve "inherit" case-insensitively.
	if raw, ok := rawMap["model"]; ok {
		var model string
		if err := json.Unmarshal(raw, &model); err == nil {
			model = strings.TrimSpace(model)
			if strings.EqualFold(model, "inherit") {
				def.Model = "inherit"
			} else if model != "" {
				def.Model = model
			}
		}
	}

	// Effort: string enum or integer.
	if raw, ok := rawMap["effort"]; ok {
		// Try string first.
		var effortStr string
		if err := json.Unmarshal(raw, &effortStr); err == nil {
			if effort := ParseEffort(effortStr); effort != "" {
				def.Effort = effort
			}
		} else {
			// Try number.
			var effortNum float64
			if err := json.Unmarshal(raw, &effortNum); err == nil {
				if effort := ParseEffort(effortNum); effort != "" {
					def.Effort = effort
				}
			}
		}
	}

	// PermissionMode: validate against known modes.
	if raw, ok := rawMap["permissionMode"]; ok {
		var pm string
		if err := json.Unmarshal(raw, &pm); err == nil {
			if slices.Contains(validPermissionModes, pm) {
				def.PermissionMode = pm
			}
		}
	}

	// MaxTurns: positive integer.
	if raw, ok := rawMap["maxTurns"]; ok {
		var maxTurns float64
		if err := json.Unmarshal(raw, &maxTurns); err == nil {
			if n := ParsePositiveInt(maxTurns); n > 0 {
				def.MaxTurns = n
			}
		}
	}

	// Background: boolean.
	if raw, ok := rawMap["background"]; ok {
		var bg bool
		if err := json.Unmarshal(raw, &bg); err == nil {
			def.Background = bg
		} else {
			var bgStr string
			if err := json.Unmarshal(raw, &bgStr); err == nil {
				def.Background = ParseBool(bgStr)
			}
		}
	}

	// Memory: validate against known scopes.
	if raw, ok := rawMap["memory"]; ok {
		var memory string
		if err := json.Unmarshal(raw, &memory); err == nil {
			if slices.Contains(validMemoryScopes, memory) {
				def.Memory = memory
			}
		}
	}

	// Isolation: validate against known modes.
	if raw, ok := rawMap["isolation"]; ok {
		var isolation string
		if err := json.Unmarshal(raw, &isolation); err == nil {
			if slices.Contains(validIsolationModes, isolation) {
				def.Isolation = isolation
			}
		}
	}

	// InitialPrompt: non-empty string after trimming.
	if raw, ok := rawMap["initialPrompt"]; ok {
		var ip string
		if err := json.Unmarshal(raw, &ip); err == nil {
			ip = strings.TrimSpace(ip)
			if ip != "" {
				def.InitialPrompt = ip
			}
		}
	}

	// Color: validate against known color names.
	if raw, ok := rawMap["color"]; ok {
		var color string
		if err := json.Unmarshal(raw, &color); err == nil {
			if c := ParseAgentColor(color); c != "" {
				def.Color = c
			}
		}
	}

	// MCPServers: parse array of server references or inline definitions.
	if raw, ok := rawMap["mcpServers"]; ok {
		var mcpArray []any
		if err := json.Unmarshal(raw, &mcpArray); err == nil {
			if servers := ParseAgentMCPServers(mcpArray); len(servers) > 0 {
				def.MCPServers = servers
			}
		}
	}

	// Hooks: parse session-scoped hooks.
	if raw, ok := rawMap["hooks"]; ok {
		var hooksMap map[string]any
		if err := json.Unmarshal(raw, &hooksMap); err == nil {
			if hooksCfg := ParseAgentHooks(hooksMap); hooksCfg != nil {
				def.Hooks = hooksCfg
			}
		}
	}

	return def, nil
}

// ParseAgentsFromJson parses multiple agent definitions from a JSON object.
// The input format is: { "agentType": { description, prompt, ... }, ... }
// Returns successfully parsed agents, skipping invalid entries.
func ParseAgentsFromJson(agentsJson json.RawMessage, source string) ([]agent.Definition, error) {
	if source == "" {
		source = "flagSettings"
	}

	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(agentsJson, &rawMap); err != nil {
		return nil, fmt.Errorf("invalid agents JSON: %w", err)
	}

	var results []agent.Definition
	for name, rawDef := range rawMap {
		def, err := ParseAgentFromJson(name, rawDef, source)
		if err != nil {
			// Silently skip invalid agents, but collect the error in the outer return.
			// Per TS behavior: log and skip.
			continue
		}
		results = append(results, def)
	}

	return results, nil
}

// parseToolsFromStrings applies the tools semantics to a string slice.
// nil/missing means all tools; empty array means no tools; ["*"] means all tools.
func parseToolsFromStrings(tools []string) []string {
	if tools == nil {
		return nil
	}
	if len(tools) == 0 {
		return []string{}
	}
	for _, t := range tools {
		if t == "*" {
			return nil
		}
	}
	return tools
}
