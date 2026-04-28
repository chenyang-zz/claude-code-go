package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const briefCommandFallback = "Brief-only mode is not available in Claude Code Go yet. Entitlement checks, Kairos gating, and tool-list sync remain unmigrated."

// BriefCommand exposes the minimum text-only /brief behavior before brief-mode runtime flows exist in the Go host.
type BriefCommand struct{}

// Metadata returns the canonical slash descriptor for /brief.
func (c BriefCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "brief",
		Description: "Toggle brief-only mode",
		Usage:       "/brief",
	}
}

// Execute accepts no arguments and reports the stable /brief fallback.
func (c BriefCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx

	raw := strings.TrimSpace(args.RawLine)
	if raw != "" {
		return command.Result{}, fmt.Errorf("usage: %s", c.Metadata().Usage)
	}

	logger.DebugCF("commands", "rendered brief command fallback output", map[string]any{
		"brief_mode_available": false,
		"arg_provided":         raw != "",
	})

	return command.Result{
		Output: briefCommandFallback,
	}, nil
}
