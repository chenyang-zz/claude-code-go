package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const debugToolCallCommandFallback = "Debug-tool-call flow is not available in Claude Code Go yet. Internal tool-call debug workflow remains unmigrated."

// DebugToolCallCommand exposes the minimum hidden /debug-tool-call behavior before internal tool-call debug flows exist in the Go host.
type DebugToolCallCommand struct{}

// Metadata returns the canonical slash descriptor for /debug-tool-call.
func (c DebugToolCallCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "debug-tool-call",
		Description: "Run internal tool-call debug workflow",
		Usage:       "/debug-tool-call",
		Hidden:      true,
	}
}

// Execute accepts no arguments and reports the stable hidden /debug-tool-call fallback.
func (c DebugToolCallCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx

	raw := strings.TrimSpace(args.RawLine)
	if raw != "" {
		return command.Result{}, fmt.Errorf("usage: %s", c.Metadata().Usage)
	}

	logger.DebugCF("commands", "rendered debug-tool-call command fallback output", map[string]any{
		"debug_tool_call_available": false,
		"hidden_command":            true,
	})

	return command.Result{
		Output: debugToolCallCommandFallback,
	}, nil
}
