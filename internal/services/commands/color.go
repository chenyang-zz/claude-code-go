package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const colorCommandFallback = "Session color customization is not available in Claude Code Go yet. Prompt bar color persistence and transcript color sync remain unmigrated."

// ColorCommand exposes the minimum text-only /color behavior before session color persistence exists in the Go runtime.
type ColorCommand struct{}

// Metadata returns the canonical slash descriptor for /color.
func (c ColorCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "color",
		Description: "Set the prompt bar color for this session",
		Usage:       "/color <color|default>",
	}
}

// Execute validates one color argument and reports the stable /color fallback supported by the current Go host.
func (c ColorCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx

	colorName := strings.TrimSpace(args.RawLine)
	if colorName == "" {
		return command.Result{}, fmt.Errorf("usage: %s", c.Metadata().Usage)
	}

	logger.DebugCF("commands", "rendered color command fallback output", map[string]any{
		"color_customization_available": false,
		"requested_color":               colorName,
	})

	return command.Result{
		Output: colorCommandFallback,
	}, nil
}
