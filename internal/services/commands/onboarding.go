package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const onboardingCommandFallback = "Onboarding command is not available in Claude Code Go yet. Interactive onboarding flow and state persistence remain unmigrated."

// OnboardingCommand exposes the minimum hidden /onboarding behavior before onboarding flows exist in the Go host.
type OnboardingCommand struct{}

// Metadata returns the canonical slash descriptor for /onboarding.
func (c OnboardingCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "onboarding",
		Description: "Run internal onboarding flow",
		Usage:       "/onboarding",
		Hidden:      true,
	}
}

// Execute accepts no arguments and reports the stable hidden /onboarding fallback.
func (c OnboardingCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx

	raw := strings.TrimSpace(args.RawLine)
	if raw != "" {
		return command.Result{}, fmt.Errorf("usage: %s", c.Metadata().Usage)
	}

	logger.DebugCF("commands", "rendered onboarding command fallback output", map[string]any{
		"onboarding_available": false,
		"hidden_command":       true,
	})

	return command.Result{
		Output: onboardingCommandFallback,
	}, nil
}
