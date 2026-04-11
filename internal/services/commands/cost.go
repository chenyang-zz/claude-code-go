package commands

import (
	"context"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// CostCommand renders the minimum text-only /cost behavior available in the Go host.
type CostCommand struct{}

// Metadata returns the canonical slash descriptor for /cost.
func (c CostCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "cost",
		Description: "Show the total cost and duration of the current session",
		Usage:       "/cost",
	}
}

// Execute reports the stable usage-tracking fallback supported by the current Go host.
func (c CostCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx
	_ = args

	logger.DebugCF("commands", "rendered cost command fallback output", map[string]any{
		"session_cost_available":     false,
		"session_duration_available": false,
	})

	return command.Result{
		Output: "Session cost and duration tracking are not available in Claude Code Go yet.",
	}, nil
}
