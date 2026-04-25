package prompts

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/agent"
)

// AgentListingSection lists available agents in the system prompt.
// It generates a markdown-formatted list of all registered agent types,
// including their purpose and available tools.
type AgentListingSection struct {
	// Registry provides the agent definitions to list.
	Registry agent.Registry
}

// Name returns the section identifier.
func (s AgentListingSection) Name() string { return "agent_listing" }

// IsVolatile reports whether this section must be recomputed every turn.
// Agent listings are stable during a session, so this returns false
// to allow caching.
func (s AgentListingSection) IsVolatile() bool { return false }

// Compute generates the agent listing section content.
// Returns an empty string when no agents are registered, which causes
// the section to be skipped by the prompt builder.
func (s AgentListingSection) Compute(ctx context.Context) (string, error) {
	if s.Registry == nil {
		return "", nil
	}

	defs := s.Registry.List()
	if len(defs) == 0 {
		return "", nil
	}

	// Sort by AgentType for deterministic output.
	sort.Slice(defs, func(i, j int) bool {
		return defs[i].AgentType < defs[j].AgentType
	})

	var lines []string
	lines = append(lines, "Available agent types for the Agent tool:")
	for _, def := range defs {
		lines = append(lines, formatAgentLine(def))
	}

	return strings.Join(lines, "\n"), nil
}

// formatAgentLine formats a single agent definition as a markdown list item.
// Format: "- {agentType}: {whenToUse} (Tools: {toolsDescription})"
func formatAgentLine(def agent.Definition) string {
	toolsDesc := getToolsDescription(def.Tools, def.DisallowedTools)
	return fmt.Sprintf("- %s: %s (Tools: %s)", def.AgentType, def.WhenToUse, toolsDesc)
}

// getToolsDescription returns a human-readable description of the tools
// available to an agent based on its allowlist and denylist.
//
// Logic:
//   - Both allowlist and denylist present: filtered intersection, "None" if empty
//   - Allowlist only: comma-separated tool names
//   - Denylist only: "All tools except X, Y, Z"
//   - Neither: "All tools"
func getToolsDescription(tools, disallowedTools []string) string {
	hasAllowlist := len(tools) > 0
	hasDenylist := len(disallowedTools) > 0

	if hasAllowlist && hasDenylist {
		denySet := make(map[string]struct{}, len(disallowedTools))
		for _, t := range disallowedTools {
			denySet[t] = struct{}{}
		}
		var effective []string
		for _, t := range tools {
			if _, denied := denySet[t]; !denied {
				effective = append(effective, t)
			}
		}
		if len(effective) == 0 {
			return "None"
		}
		return strings.Join(effective, ", ")
	} else if hasAllowlist {
		return strings.Join(tools, ", ")
	} else if hasDenylist {
		return fmt.Sprintf("All tools except %s", strings.Join(disallowedTools, ", "))
	}
	return "All tools"
}
