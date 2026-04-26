package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const thinkBackCommandFallback = "Think-back yearly review is not available in Claude Code Go yet. Feature-gated analytics generation and review UI playback remain unmigrated."

// ThinkBackCommand exposes the minimum text-only /think-back behavior before review generation flows exist in the Go host.
type ThinkBackCommand struct{}

// Metadata returns the canonical slash descriptor for /think-back.
func (c ThinkBackCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "think-back",
		Description: "Your 2025 Claude Code Year in Review",
		Usage:       "/think-back",
	}
}

// Execute accepts no arguments and reports the stable /think-back fallback.
func (c ThinkBackCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx

	raw := strings.TrimSpace(args.RawLine)
	if raw != "" {
		return command.Result{}, fmt.Errorf("usage: %s", c.Metadata().Usage)
	}

	logger.DebugCF("commands", "rendered think-back command fallback output", map[string]any{
		"think_back_available": false,
	})

	return command.Result{
		Output: thinkBackCommandFallback,
	}, nil
}
