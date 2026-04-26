package commands

import (
	"context"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const ultrareviewCommandFallback = "Ultrareview is not available in Claude Code Go yet. Remote web review orchestration and overage flows remain unmigrated."

// UltrareviewCommand exposes the minimum text-only /ultrareview behavior before remote ultrareview flows exist in the Go host.
type UltrareviewCommand struct{}

// Metadata returns the canonical slash descriptor for /ultrareview.
func (c UltrareviewCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "ultrareview",
		Description: "Find and verify bugs in your branch using Claude Code on the web",
		Usage:       "/ultrareview [pr-number]",
	}
}

// Execute accepts optional arguments and reports the stable /ultrareview fallback.
func (c UltrareviewCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx

	raw := strings.TrimSpace(args.RawLine)

	logger.DebugCF("commands", "rendered ultrareview command fallback output", map[string]any{
		"ultrareview_available": false,
		"arg_provided":          raw != "",
	})

	return command.Result{
		Output: ultrareviewCommandFallback,
	}, nil
}
