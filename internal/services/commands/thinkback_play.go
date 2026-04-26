package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const thinkbackPlayCommandFallback = "Thinkback animation playback is not available in Claude Code Go yet. Hidden animation command wiring and playback renderer remain unmigrated."

// ThinkbackPlayCommand exposes the minimum text-only hidden /thinkback-play behavior.
type ThinkbackPlayCommand struct{}

// Metadata returns the canonical slash descriptor for /thinkback-play.
func (c ThinkbackPlayCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "thinkback-play",
		Description: "Play the thinkback animation",
		Usage:       "/thinkback-play",
		Hidden:      true,
	}
}

// Execute accepts no arguments and reports the stable hidden /thinkback-play fallback.
func (c ThinkbackPlayCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx

	raw := strings.TrimSpace(args.RawLine)
	if raw != "" {
		return command.Result{}, fmt.Errorf("usage: %s", c.Metadata().Usage)
	}

	logger.DebugCF("commands", "rendered thinkback-play command fallback output", map[string]any{
		"thinkback_play_available": false,
		"hidden_command":           true,
	})

	return command.Result{
		Output: thinkbackPlayCommandFallback,
	}, nil
}
