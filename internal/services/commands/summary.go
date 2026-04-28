package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const summaryCommandFallback = "Summary command is not available in Claude Code Go yet. Internal post-turn summary and report rendering remain unmigrated."

// SummaryCommand exposes the minimum hidden /summary behavior before summary flows exist in the Go host.
type SummaryCommand struct{}

// Metadata returns the canonical slash descriptor for /summary.
func (c SummaryCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "summary",
		Description: "Show internal summary diagnostics",
		Usage:       "/summary",
		Hidden:      true,
	}
}

// Execute accepts no arguments and reports the stable hidden /summary fallback.
func (c SummaryCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx

	raw := strings.TrimSpace(args.RawLine)
	if raw != "" {
		return command.Result{}, fmt.Errorf("usage: %s", c.Metadata().Usage)
	}

	logger.DebugCF("commands", "rendered summary command fallback output", map[string]any{
		"summary_available": false,
		"hidden_command":    true,
	})

	return command.Result{
		Output: summaryCommandFallback,
	}, nil
}
