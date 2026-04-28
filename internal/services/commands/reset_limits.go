package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const resetLimitsCommandFallback = "Reset-limits flow is not available in Claude Code Go yet. Internal reset-limits workflow remains unmigrated."

// ResetLimitsCommand exposes the minimum hidden /reset-limits behavior before internal reset-limits flows exist in the Go host.
type ResetLimitsCommand struct{}

// Metadata returns the canonical slash descriptor for /reset-limits.
func (c ResetLimitsCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "reset-limits",
		Aliases:     []string{"reset-limits-non-interactive"},
		Description: "Reset internal usage limits",
		Usage:       "/reset-limits",
		Hidden:      true,
	}
}

// Execute accepts no arguments and reports the stable hidden /reset-limits fallback.
func (c ResetLimitsCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx

	raw := strings.TrimSpace(args.RawLine)
	if raw != "" {
		return command.Result{}, fmt.Errorf("usage: %s", c.Metadata().Usage)
	}

	logger.DebugCF("commands", "rendered reset-limits command fallback output", map[string]any{
		"reset_limits_available": false,
		"hidden_command":         true,
	})

	return command.Result{
		Output: resetLimitsCommandFallback,
	}, nil
}
