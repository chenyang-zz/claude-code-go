package commands

import (
	"context"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const rateLimitOptionsCommandFallback = "Rate limit options flow is not available in Claude Code Go yet. Upgrade and extra-usage selection UX remain unmigrated."

// RateLimitOptionsCommand exposes the minimum text-only /rate-limit-options behavior before rate-limit recovery UX exists in the Go runtime.
type RateLimitOptionsCommand struct{}

// Metadata returns the canonical slash descriptor for /rate-limit-options.
func (c RateLimitOptionsCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "rate-limit-options",
		Description: "Show options when rate limit is reached",
		Usage:       "/rate-limit-options",
		Hidden:      true,
	}
}

// Execute reports the stable hidden /rate-limit-options fallback supported by the current Go host.
func (c RateLimitOptionsCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx
	_ = args

	logger.DebugCF("commands", "rendered rate-limit-options command fallback output", map[string]any{
		"rate_limit_options_available": false,
		"hidden_command":               true,
	})

	return command.Result{
		Output: rateLimitOptionsCommandFallback,
	}, nil
}
