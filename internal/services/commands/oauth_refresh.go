package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const oauthRefreshCommandFallback = "OAuth refresh command is not available in Claude Code Go yet. Refresh token exchange and account sync flows remain unmigrated."

// OAuthRefreshCommand exposes the minimum hidden /oauth-refresh behavior before OAuth refresh flows exist in the Go host.
type OAuthRefreshCommand struct{}

// Metadata returns the canonical slash descriptor for /oauth-refresh.
func (c OAuthRefreshCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "oauth-refresh",
		Description: "Refresh OAuth credentials",
		Usage:       "/oauth-refresh",
		Hidden:      true,
	}
}

// Execute accepts no arguments and reports the stable hidden /oauth-refresh fallback.
func (c OAuthRefreshCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx

	raw := strings.TrimSpace(args.RawLine)
	if raw != "" {
		return command.Result{}, fmt.Errorf("usage: %s", c.Metadata().Usage)
	}

	logger.DebugCF("commands", "rendered oauth-refresh command fallback output", map[string]any{
		"oauth_refresh_available": false,
		"hidden_command":          true,
	})

	return command.Result{
		Output: oauthRefreshCommandFallback,
	}, nil
}
