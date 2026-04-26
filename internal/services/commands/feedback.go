package commands

import (
	"context"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const feedbackCommandFallback = "Product feedback submission is not available in Claude Code Go yet. Interactive feedback forms, privacy-aware routing, and upstream report delivery remain unmigrated."

// FeedbackCommand exposes the minimum text-only /feedback behavior before feedback submission flows exist in the Go runtime.
type FeedbackCommand struct{}

// Metadata returns the canonical slash descriptor for /feedback.
func (c FeedbackCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "feedback",
		Aliases:     []string{"bug"},
		Description: "Submit feedback about Claude Code",
		Usage:       "/feedback [report]",
	}
}

// Execute reports the stable /feedback fallback supported by the current Go host.
func (c FeedbackCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx
	_ = args

	logger.DebugCF("commands", "rendered feedback command fallback output", map[string]any{
		"feedback_submission_available": false,
	})

	return command.Result{
		Output: feedbackCommandFallback,
	}, nil
}
