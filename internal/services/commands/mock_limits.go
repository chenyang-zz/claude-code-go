package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const mockLimitsCommandFallback = "Mock-limits flow is not available in Claude Code Go yet. Internal mock-limits workflow remains unmigrated."

// MockLimitsCommand exposes the minimum hidden /mock-limits behavior before internal mock-limits flows exist in the Go host.
type MockLimitsCommand struct{}

// Metadata returns the canonical slash descriptor for /mock-limits.
func (c MockLimitsCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "mock-limits",
		Description: "Mock internal usage limits",
		Usage:       "/mock-limits",
		Hidden:      true,
	}
}

// Execute accepts no arguments and reports the stable hidden /mock-limits fallback.
func (c MockLimitsCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx

	raw := strings.TrimSpace(args.RawLine)
	if raw != "" {
		return command.Result{}, fmt.Errorf("usage: %s", c.Metadata().Usage)
	}

	logger.DebugCF("commands", "rendered mock-limits command fallback output", map[string]any{
		"mock_limits_available": false,
		"hidden_command":        true,
	})

	return command.Result{
		Output: mockLimitsCommandFallback,
	}, nil
}
