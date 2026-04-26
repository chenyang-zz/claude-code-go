package commands

import (
	"context"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const initCommandFallback = "Project initialization flow is not available in Claude Code Go yet. Interactive CLAUDE.md bootstrapping, optional skill/hook setup, and guided onboarding prompts remain unmigrated."

// InitCommand exposes the minimum text-only /init behavior before guided initialization exists in the Go runtime.
type InitCommand struct{}

// Metadata returns the canonical slash descriptor for /init.
func (c InitCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "init",
		Description: "Initialize a new CLAUDE.md file with codebase documentation",
		Usage:       "/init",
	}
}

// Execute reports the stable /init fallback supported by the current Go host.
func (c InitCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx
	_ = args

	logger.DebugCF("commands", "rendered init command fallback output", map[string]any{
		"project_init_available": false,
	})

	return command.Result{
		Output: initCommandFallback,
	}, nil
}
