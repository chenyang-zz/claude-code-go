package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const envCommandFallback = "Environment diagnostics command is not available in Claude Code Go yet. Internal environment inspection and mutation flows remain unmigrated."

// EnvCommand exposes the minimum hidden /env behavior before environment flows exist in the Go host.
type EnvCommand struct{}

// Metadata returns the canonical slash descriptor for /env.
func (c EnvCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "env",
		Description: "Inspect internal environment state",
		Usage:       "/env",
		Hidden:      true,
	}
}

// Execute accepts no arguments and reports the stable hidden /env fallback.
func (c EnvCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx

	raw := strings.TrimSpace(args.RawLine)
	if raw != "" {
		return command.Result{}, fmt.Errorf("usage: %s", c.Metadata().Usage)
	}

	logger.DebugCF("commands", "rendered env command fallback output", map[string]any{
		"env_available":  false,
		"hidden_command": true,
	})

	return command.Result{
		Output: envCommandFallback,
	}, nil
}
