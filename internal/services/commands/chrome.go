package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const chromeCommandFallback = "Claude in Chrome settings is not available in Claude Code Go yet. Subscription gating, extension setup, and browser control flows remain unmigrated."

// ChromeCommand exposes the minimum text-only /chrome behavior before Chrome integration flows exist in the Go host.
type ChromeCommand struct{}

// Metadata returns the canonical slash descriptor for /chrome.
func (c ChromeCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "chrome",
		Description: "Claude in Chrome (Beta) settings",
		Usage:       "/chrome",
	}
}

// Execute accepts no arguments and reports the stable /chrome fallback.
func (c ChromeCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx

	raw := strings.TrimSpace(args.RawLine)
	if raw != "" {
		return command.Result{}, fmt.Errorf("usage: %s", c.Metadata().Usage)
	}

	logger.DebugCF("commands", "rendered chrome command fallback output", map[string]any{
		"chrome_integration_available": false,
	})

	return command.Result{
		Output: chromeCommandFallback,
	}, nil
}
