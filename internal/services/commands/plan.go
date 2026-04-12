package commands

import (
	"context"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const planCommandFallback = "Plan mode is not available in Claude Code Go yet. Enabling plan mode, viewing the current plan, opening the plan file in an editor, and plan-file lifecycle management remain unmigrated."

// PlanCommand exposes the minimum text-only /plan behavior before plan mode and plan-file workflows exist in the Go host.
type PlanCommand struct{}

// Metadata returns the canonical slash descriptor for /plan.
func (c PlanCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "plan",
		Description: "Enable plan mode or view the current session plan",
		Usage:       "/plan [open|<description>]",
	}
}

// Execute reports the stable /plan fallback supported by the current Go host.
func (c PlanCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx
	_ = args

	logger.DebugCF("commands", "rendered plan command fallback output", map[string]any{
		"plan_mode_available":   false,
		"plan_view_available":   false,
		"plan_editor_available": false,
	})

	return command.Result{
		Output: planCommandFallback,
	}, nil
}
