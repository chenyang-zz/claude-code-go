package commands

import (
	"context"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const statuslineCommandFallback = "Status line setup is not available in Claude Code Go yet. Agent-driven statusline configuration and settings file updates remain unmigrated."

// StatuslineCommand exposes the minimum text-only /statusline behavior before statusline setup flows exist in the Go host.
type StatuslineCommand struct{}

// Metadata returns the canonical slash descriptor for /statusline.
func (c StatuslineCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "statusline",
		Description: "Set up Claude Code's status line UI",
		Usage:       "/statusline [prompt]",
	}
}

// Execute accepts optional arguments and reports the stable /statusline fallback.
func (c StatuslineCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx

	raw := strings.TrimSpace(args.RawLine)

	logger.DebugCF("commands", "rendered statusline command fallback output", map[string]any{
		"statusline_setup_available": false,
		"arg_provided":               raw != "",
	})

	return command.Result{
		Output: statuslineCommandFallback,
	}, nil
}
