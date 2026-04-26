package commands

import (
	"context"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const contextCommandFallback = "Context visualization is not available in Claude Code Go yet. Colored context grid rendering, token-accurate compacted analysis, and interactive breakdown views remain unmigrated."

// ContextCommand exposes the minimum text-only /context behavior before context visualization features exist in the Go runtime.
type ContextCommand struct{}

// Metadata returns the canonical slash descriptor for /context.
func (c ContextCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "context",
		Description: "Show current context usage",
		Usage:       "/context",
	}
}

// Execute reports the stable /context fallback supported by the current Go host.
func (c ContextCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx
	_ = args

	logger.DebugCF("commands", "rendered context command fallback output", map[string]any{
		"context_visualization_available": false,
	})

	return command.Result{
		Output: contextCommandFallback,
	}, nil
}
