package commands

import (
	"context"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// LoginCommand renders the minimum text-only /login behavior available in the Go host.
type LoginCommand struct {
	// Config carries the already-resolved runtime configuration snapshot.
	Config coreconfig.Config
}

// Metadata returns the canonical slash descriptor for /login.
func (c LoginCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "login",
		Description: "Sign in with your Anthropic account",
		Usage:       "/login",
	}
}

// Execute reports the stable authentication guidance supported by the current Go host.
func (c LoginCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx
	_ = args

	usingAPIKey := strings.TrimSpace(c.Config.APIKey) != ""
	logger.DebugCF("commands", "rendered login command fallback output", map[string]any{
		"using_api_key": usingAPIKey,
	})

	if usingAPIKey {
		return command.Result{
			Output: "Claude Code Go is using configured API key authentication. Interactive account switching is not supported yet.",
		}, nil
	}

	return command.Result{
		Output: "Interactive Anthropic account login is not supported in Claude Code Go yet. Configure an API key in settings or environment variables instead.",
	}, nil
}
