package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const shareCommandFallback = "Share flow is not available in Claude Code Go yet. Session publishing and share link generation remain unmigrated."

// ShareCommand exposes the minimum hidden /share behavior before sharing flows exist in the Go host.
type ShareCommand struct{}

// Metadata returns the canonical slash descriptor for /share.
func (c ShareCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "share",
		Description: "Share the current session",
		Usage:       "/share",
		Hidden:      true,
	}
}

// Execute accepts no arguments and reports the stable hidden /share fallback.
func (c ShareCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx

	raw := strings.TrimSpace(args.RawLine)
	if raw != "" {
		return command.Result{}, fmt.Errorf("usage: %s", c.Metadata().Usage)
	}

	logger.DebugCF("commands", "rendered share command fallback output", map[string]any{
		"share_available": false,
		"hidden_command":  true,
	})

	return command.Result{
		Output: shareCommandFallback,
	}, nil
}
