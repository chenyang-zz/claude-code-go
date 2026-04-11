package commands

import (
	"context"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// LogoutCommand renders the minimum text-only /logout behavior available in the Go host.
type LogoutCommand struct {
	// Config carries the already-resolved runtime configuration snapshot.
	Config coreconfig.Config
}

// Metadata returns the canonical slash descriptor for /logout.
func (c LogoutCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "logout",
		Description: "Sign out from your Anthropic account",
		Usage:       "/logout",
	}
}

// Execute reports the stable logout guidance supported by the current Go host.
func (c LogoutCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx
	_ = args

	usingAPIKey := strings.TrimSpace(c.Config.APIKey) != ""
	logger.DebugCF("commands", "rendered logout command fallback output", map[string]any{
		"using_api_key": usingAPIKey,
	})

	if usingAPIKey {
		return command.Result{
			Output: "Claude Code Go is using configured API key authentication. Remove the API key from settings or environment variables to sign out.",
		}, nil
	}

	return command.Result{
		Output: "Interactive Anthropic account logout is not supported in Claude Code Go yet because account login is not available.",
	}, nil
}
