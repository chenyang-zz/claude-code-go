package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const antTraceCommandFallback = "ANT trace flow is not available in Claude Code Go yet. Internal ant-trace workflow remains unmigrated."

// AntTraceCommand exposes the minimum hidden /ant-trace behavior before internal ant-trace flows exist in the Go host.
type AntTraceCommand struct{}

// Metadata returns the canonical slash descriptor for /ant-trace.
func (c AntTraceCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "ant-trace",
		Description: "Run internal ANT trace workflow",
		Usage:       "/ant-trace",
		Hidden:      true,
	}
}

// Execute accepts no arguments and reports the stable hidden /ant-trace fallback.
func (c AntTraceCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx

	raw := strings.TrimSpace(args.RawLine)
	if raw != "" {
		return command.Result{}, fmt.Errorf("usage: %s", c.Metadata().Usage)
	}

	logger.DebugCF("commands", "rendered ant-trace command fallback output", map[string]any{
		"ant_trace_available": false,
		"hidden_command":      true,
	})

	return command.Result{
		Output: antTraceCommandFallback,
	}, nil
}
