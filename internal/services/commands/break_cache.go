package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const breakCacheCommandFallback = "Break-cache flow is not available in Claude Code Go yet. Internal cache reset workflow remains unmigrated."

// BreakCacheCommand exposes the minimum hidden /break-cache behavior before cache reset flows exist in the Go host.
type BreakCacheCommand struct{}

// Metadata returns the canonical slash descriptor for /break-cache.
func (c BreakCacheCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "break-cache",
		Description: "Run internal cache-break workflow",
		Usage:       "/break-cache",
		Hidden:      true,
	}
}

// Execute accepts no arguments and reports the stable hidden /break-cache fallback.
func (c BreakCacheCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx

	raw := strings.TrimSpace(args.RawLine)
	if raw != "" {
		return command.Result{}, fmt.Errorf("usage: %s", c.Metadata().Usage)
	}

	logger.DebugCF("commands", "rendered break-cache command fallback output", map[string]any{
		"break_cache_available": false,
		"hidden_command":        true,
	})

	return command.Result{
		Output: breakCacheCommandFallback,
	}, nil
}
