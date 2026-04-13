package commands

import (
	"context"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const hooksCommandFallback = "Hook configuration is not available in Claude Code Go yet. Tool event hook menus, hook rule editing, and interactive hook configuration flows remain unmigrated."

// HooksCommand exposes the minimum text-only /hooks behavior before hook configuration UI exists in the Go runtime.
type HooksCommand struct{}

// Metadata returns the canonical slash descriptor for /hooks.
func (c HooksCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "hooks",
		Description: "View hook configurations for tool events",
		Usage:       "/hooks",
	}
}

// Execute reports the stable /hooks fallback supported by the current Go host.
func (c HooksCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx
	_ = args

	logger.DebugCF("commands", "rendered hooks command fallback output", map[string]any{
		"hook_configuration_available": false,
	})

	return command.Result{
		Output: hooksCommandFallback,
	}, nil
}
