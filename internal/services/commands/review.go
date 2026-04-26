package commands

import (
	"context"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const reviewCommandFallback = "Pull request review workflow is not available in Claude Code Go yet. Prompt-driven PR analysis flow and remote ultrareview integration remain unmigrated."

// ReviewCommand exposes the minimum text-only /review behavior before PR review workflows exist in the Go runtime.
type ReviewCommand struct{}

// Metadata returns the canonical slash descriptor for /review.
func (c ReviewCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "review",
		Description: "Review a pull request",
		Usage:       "/review [pr-number]",
	}
}

// Execute reports the stable /review fallback supported by the current Go host.
func (c ReviewCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx
	_ = args

	logger.DebugCF("commands", "rendered review command fallback output", map[string]any{
		"review_workflow_available": false,
	})

	return command.Result{
		Output: reviewCommandFallback,
	}, nil
}
