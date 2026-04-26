package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const sandboxCommandFallback = "Sandbox configuration is not available in Claude Code Go yet. Platform dependency checks, managed policy evaluation, and interactive sandbox settings remain unmigrated."

// SandboxCommand exposes the minimum text-only /sandbox behavior before sandbox configuration flows exist in the Go runtime.
type SandboxCommand struct{}

// Metadata returns the canonical slash descriptor for /sandbox.
func (c SandboxCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "sandbox",
		Description: "Configure sandbox settings",
		Usage:       "/sandbox [exclude <command-pattern>]",
	}
}

// Execute accepts either no arguments or the exclude subcommand and reports the stable /sandbox fallback.
func (c SandboxCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx

	raw := strings.TrimSpace(args.RawLine)
	if raw != "" {
		parts := strings.Fields(raw)
		if len(parts) == 0 || parts[0] != "exclude" {
			return command.Result{}, fmt.Errorf("usage: %s", c.Metadata().Usage)
		}
		pattern := strings.TrimSpace(strings.TrimPrefix(raw, "exclude"))
		if pattern == "" {
			return command.Result{}, fmt.Errorf("usage: %s", c.Metadata().Usage)
		}
	}

	logger.DebugCF("commands", "rendered sandbox command fallback output", map[string]any{
		"sandbox_configuration_available": false,
		"raw_args":                        raw,
	})

	return command.Result{
		Output: sandboxCommandFallback,
	}, nil
}
