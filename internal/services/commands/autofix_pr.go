package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const autofixPRCommandFallback = "Autofix-pr flow is not available in Claude Code Go yet. Internal autofix PR workflow remains unmigrated."

// AutofixPRCommand exposes the minimum hidden /autofix-pr behavior before internal autofix PR flows exist in the Go host.
type AutofixPRCommand struct{}

// Metadata returns the canonical slash descriptor for /autofix-pr.
func (c AutofixPRCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "autofix-pr",
		Description: "Run internal autofix PR workflow",
		Usage:       "/autofix-pr",
		Hidden:      true,
	}
}

// Execute accepts no arguments and reports the stable hidden /autofix-pr fallback.
func (c AutofixPRCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx

	raw := strings.TrimSpace(args.RawLine)
	if raw != "" {
		return command.Result{}, fmt.Errorf("usage: %s", c.Metadata().Usage)
	}

	logger.DebugCF("commands", "rendered autofix-pr command fallback output", map[string]any{
		"autofix_pr_available": false,
		"hidden_command":       true,
	})

	return command.Result{
		Output: autofixPRCommandFallback,
	}, nil
}
