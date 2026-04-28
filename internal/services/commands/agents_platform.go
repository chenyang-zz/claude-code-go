package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const agentsPlatformCommandFallback = "Agents-platform flow is not available in Claude Code Go yet. Internal agents-platform workflow remains unmigrated."

// AgentsPlatformCommand exposes the minimum hidden /agents-platform behavior before internal agents-platform flows exist in the Go host.
type AgentsPlatformCommand struct{}

// Metadata returns the canonical slash descriptor for /agents-platform.
func (c AgentsPlatformCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "agents-platform",
		Description: "Run internal agents-platform workflow",
		Usage:       "/agents-platform",
		Hidden:      true,
	}
}

// Execute accepts no arguments and reports the stable hidden /agents-platform fallback.
func (c AgentsPlatformCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx

	raw := strings.TrimSpace(args.RawLine)
	if raw != "" {
		return command.Result{}, fmt.Errorf("usage: %s", c.Metadata().Usage)
	}

	logger.DebugCF("commands", "rendered agents-platform command fallback output", map[string]any{
		"agents_platform_available": false,
		"hidden_command":            true,
	})

	return command.Result{
		Output: agentsPlatformCommandFallback,
	}, nil
}
