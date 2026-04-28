package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const backfillSessionsCommandFallback = "Backfill-sessions flow is not available in Claude Code Go yet. Internal session backfill remains unmigrated."

// BackfillSessionsCommand exposes the minimum hidden /backfill-sessions behavior before session backfill flows exist in the Go host.
type BackfillSessionsCommand struct{}

// Metadata returns the canonical slash descriptor for /backfill-sessions.
func (c BackfillSessionsCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "backfill-sessions",
		Description: "Backfill internal session fixtures",
		Usage:       "/backfill-sessions",
		Hidden:      true,
	}
}

// Execute accepts no arguments and reports the stable hidden /backfill-sessions fallback.
func (c BackfillSessionsCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx

	raw := strings.TrimSpace(args.RawLine)
	if raw != "" {
		return command.Result{}, fmt.Errorf("usage: %s", c.Metadata().Usage)
	}

	logger.DebugCF("commands", "rendered backfill-sessions command fallback output", map[string]any{
		"backfill_sessions_available": false,
		"hidden_command":              true,
	})

	return command.Result{
		Output: backfillSessionsCommandFallback,
	}, nil
}
