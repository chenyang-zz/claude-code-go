package agent

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/agent"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// Descriptor generates dynamic tool descriptions for the Agent tool.
// It queries the agent registry to produce a description that includes
// available agent types, their capabilities, usage guidelines, and examples.
type Descriptor struct {
	// Registry provides the agent definitions used to build the description.
	Registry agent.Registry
}

// Description returns the full dynamic description string.
// When the registry is nil or empty, it falls back to a simplified static description.
func (d *Descriptor) Description() string {
	if d.Registry == nil {
		logger.DebugCF("agent.descriptor", "registry is nil, returning fallback description", nil)
		return fallbackDescription()
	}

	defs := d.Registry.List()
	if len(defs) == 0 {
		logger.DebugCF("agent.descriptor", "registry is empty, returning fallback description", nil)
		return fallbackDescription()
	}

	// Sort by AgentType for deterministic output.
	sort.Slice(defs, func(i, j int) bool {
		return defs[i].AgentType < defs[j].AgentType
	})

	logger.InfoCF("agent.descriptor", "generating dynamic description", map[string]any{
		"agent_count": len(defs),
	})

	var b strings.Builder

	b.WriteString("Launch a specialized agent to handle complex, multi-step tasks autonomously.\n\n")
	b.WriteString("The Agent tool launches specialized agents (subprocesses) that autonomously handle complex tasks. ")
	b.WriteString("Each agent type has specific capabilities and tools available to it.\n\n")

	b.WriteString("Available agent types and the tools they have access to:\n")
	for _, def := range defs {
		b.WriteString(formatAgentLine(def))
		b.WriteByte('\n')
	}

	b.WriteString("\nWhen NOT to use the Agent tool:\n")
	b.WriteString("- If you want to read a specific file path, use the Read tool or the Bash tool instead of the Agent tool, to find the match more quickly\n")
	b.WriteString("- If you are searching for code within a specific file or set of 2-3 files, use the Read tool instead of the Agent tool, to find the match more quickly\n")
	b.WriteString("- Other tasks that are not related to the agent descriptions above\n")

	b.WriteString("\nUsage notes:\n")
	b.WriteString("- Always include a short description (3-5 words) summarizing what the agent will do\n")
	b.WriteString("- When the agent is done, it will return a single message back to you. ")
	b.WriteString("The result returned by the agent is not visible to the user. ")
	b.WriteString("To show the user the result, you should send a text message back to the user with a concise summary of the result.\n")
	b.WriteString("- Each Agent invocation starts fresh — provide a complete task description.\n")
	b.WriteString("- The agent's outputs should generally be trusted\n")
	b.WriteString("- Clearly tell the agent whether you expect it to write code or just to do research (search, file reads, web fetches, etc.), since it is not aware of the user's intent\n")
	b.WriteString("- If the agent description mentions that it should be used proactively, then you should try your best to use it without the user having to ask for it first. Use your judgement.\n")
	b.WriteString("- If the user specifies that they want you to run agents \"in parallel\", you MUST send a single message with multiple Agent tool use content blocks. ")
	b.WriteString("For example, if you need to launch both a build-validator agent and a test-runner agent in parallel, send a single message with both tool calls.\n")

	b.WriteString("\nExample usage:\n\n")
	b.WriteString("<example_agent_descriptions>\n")
	b.WriteString("\"test-runner\": use this agent after you are done writing code to run tests\n")
	b.WriteString("</example_agent_descriptions>\n\n")
	b.WriteString("<example>\n")
	b.WriteString("user: \"Please write a function that checks if a number is prime\"\n")
	b.WriteString("assistant: I'm going to use the Write tool to write the following code:\n")
	b.WriteString("...\n")
	b.WriteString("<commentary>\n")
	b.WriteString("Since a significant piece of code was written and the task was completed, now use the test-runner agent to run the tests\n")
	b.WriteString("</commentary>\n")
	b.WriteString("assistant: Uses the Agent tool to launch the test-runner agent\n")
	b.WriteString("</example>")

	return b.String()
}

// fallbackDescription returns a simplified static description when no registry
// information is available.
func fallbackDescription() string {
	return "Launch a specialized agent to perform a task. Use this when you need to delegate work to a subagent."
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
