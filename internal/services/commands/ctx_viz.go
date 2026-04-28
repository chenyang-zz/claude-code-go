package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const ctxVizCommandFallback = "Ctx_viz flow is not available in Claude Code Go yet. Internal context visualization remains unmigrated."

// CtxVizCommand exposes the minimum hidden /ctx_viz behavior before visualization flows exist in the Go host.
type CtxVizCommand struct{}

// Metadata returns the canonical slash descriptor for /ctx_viz.
func (c CtxVizCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "ctx_viz",
		Description: "Run internal context visualization workflow",
		Usage:       "/ctx_viz",
		Hidden:      true,
	}
}

// Execute accepts no arguments and reports the stable hidden /ctx_viz fallback.
func (c CtxVizCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx

	raw := strings.TrimSpace(args.RawLine)
	if raw != "" {
		return command.Result{}, fmt.Errorf("usage: %s", c.Metadata().Usage)
	}

	logger.DebugCF("commands", "rendered ctx_viz command fallback output", map[string]any{
		"ctx_viz_available": false,
		"hidden_command":    true,
	})

	return command.Result{
		Output: ctxVizCommandFallback,
	}, nil
}
