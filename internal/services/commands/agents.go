package commands

import (
	"context"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const agentsCommandFallback = "Agent configuration is not available in Claude Code Go yet. Agent menus, team configuration editing, and interactive agent management flows remain unmigrated."

// AgentsCommand exposes the minimum text-only /agents behavior before agent management UI exists in the Go runtime.
type AgentsCommand struct{}

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

	logger.DebugCF("commands", "rendered agents command fallback output", map[string]any{
		"agent_configuration_available": false,
	})

	return command.Result{
		Output: agentsCommandFallback,
	}, nil
}
