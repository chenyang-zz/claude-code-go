package commands

import (
	"context"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const passesCommandFallback = "Guest passes is not available in Claude Code Go yet. Referral eligibility checks, pass tracking, and passes UI flow remain unmigrated."

// PassesCommand exposes the minimum text-only /passes behavior before referral features exist in the Go runtime.
type PassesCommand struct{}

// Metadata returns the canonical slash descriptor for /passes.
func (c PassesCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "passes",
		Description: "Share a free week of Claude Code with friends",
		Usage:       "/passes",
	}
}

// Execute reports the stable /passes fallback supported by the current Go host.
func (c PassesCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx
	_ = args

	logger.DebugCF("commands", "rendered passes command fallback output", map[string]any{
		"passes_feature_available": false,
	})

	return command.Result{
		Output: passesCommandFallback,
	}, nil
}
