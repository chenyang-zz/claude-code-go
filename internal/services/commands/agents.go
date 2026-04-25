package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/agent"
	"github.com/sheepzhao/claude-code-go/internal/core/command"
	platformteam "github.com/sheepzhao/claude-code-go/internal/platform/team"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const agentsCommandFallback = "Agent configuration is not available in Claude Code Go yet. Agent menus, team configuration editing, and interactive agent management flows remain unmigrated."

// TeamStatusProvider exposes the current team status summary used by /agents.
type TeamStatusProvider interface {
	// CurrentTeamStatus returns the current team summary, or nil when no team config exists.
	CurrentTeamStatus(ctx context.Context) (*platformteam.Status, error)
}

// AgentsCommand exposes the minimum text-only /agents behavior before agent management UI exists in the Go runtime.
type AgentsCommand struct {
	// StatusProvider supplies the current team summary for read-only rendering.
	StatusProvider TeamStatusProvider
}

// Metadata returns the canonical slash descriptor for /agents.
func (c AgentsCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "agents",
		Description: "Manage agent configurations",
		Usage:       "/agents",
	}
}

// Execute reports the stable /agents fallback supported by the current Go host.
func (c AgentsCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx
	_ = args

	if c.StatusProvider == nil {
		logger.DebugCF("commands", "rendered agents command fallback output", map[string]any{
			"agent_configuration_available": false,
		})
		return command.Result{
			Output: agentsCommandFallback,
		}, nil
	}

	status, err := c.StatusProvider.CurrentTeamStatus(ctx)
	if err != nil {
		logger.DebugCF("commands", "failed to load agents command status", map[string]any{
			"error": err.Error(),
		})
		return command.Result{
			Output: agentsCommandFallback,
		}, nil
	}
	if status == nil {
		return command.Result{
			Output: "No team configuration found.\nAgent status summary is not available in this session.",
		}, nil
	}

	output := renderAgentsOutput(status)
	logger.DebugCF("commands", "rendered agents command fallback output", map[string]any{
		"agent_configuration_available": true,
		"team_name":                     status.TeamName,
		"member_count":                  len(status.Members),
	})

	return command.Result{
		Output: output,
	}, nil
}

// renderAgentsOutput formats a team status summary into a stable read-only text block.
func renderAgentsOutput(status *platformteam.Status) string {
	if status == nil {
		return agentsCommandFallback
	}

	lines := []string{
		"Agent status summary:",
		fmt.Sprintf("Team: %s", displayValue(status.TeamName)),
	}
	if status.LeadAgentID != "" {
		lines = append(lines, fmt.Sprintf("Lead agent ID: %s", status.LeadAgentID))
	}
	if len(status.Members) == 0 {
		lines = append(lines, "No team members found.")
		return strings.Join(lines, "\n")
	}

	for _, member := range status.Members {
		lines = append(lines, renderAgentStatusLine(member))
	}
	return strings.Join(lines, "\n")
}

// renderAgentStatusLine formats one agent status row for /agents.
func renderAgentStatusLine(status agent.Status) string {
	line := fmt.Sprintf("- %s", displayValue(status.Name))
	if strings.TrimSpace(status.AgentType) != "" {
		line += fmt.Sprintf(" (%s)", status.AgentType)
	}
	line += fmt.Sprintf(": %s", status.Status)
	if len(status.CurrentTasks) > 0 {
		line += fmt.Sprintf(" [%s]", strings.Join(status.CurrentTasks, ", "))
	}
	return line
}
