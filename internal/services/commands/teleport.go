package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const teleportCommandFallback = "Teleport command is not available in Claude Code Go yet. Remote handoff and teleport session flows remain unmigrated."

// TeleportCommand exposes the minimum hidden /teleport behavior before teleport flows exist in the Go host.
type TeleportCommand struct{}

// Metadata returns the canonical slash descriptor for /teleport.
func (c TeleportCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "teleport",
		Description: "Teleport the current session to remote runtime",
		Usage:       "/teleport",
		Hidden:      true,
	}
}

// Execute accepts no arguments and reports the stable hidden /teleport fallback.
func (c TeleportCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx

	raw := strings.TrimSpace(args.RawLine)
	if raw != "" {
		return command.Result{}, fmt.Errorf("usage: %s", c.Metadata().Usage)
	}

	logger.DebugCF("commands", "rendered teleport command fallback output", map[string]any{
		"teleport_available": false,
		"hidden_command":     true,
	})

	return command.Result{
		Output: teleportCommandFallback,
	}, nil
}
