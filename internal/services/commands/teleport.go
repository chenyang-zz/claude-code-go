package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/internal/core/featureflag"
	"github.com/sheepzhao/claude-code-go/internal/services/teleport"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// TeleportCommand exposes the /teleport slash command for remote session management.
type TeleportCommand struct {
	// Service is the teleport service instance, injected at bootstrap.
	Service *teleport.TeleportService
}

// Metadata returns the canonical slash descriptor for /teleport.
func (c TeleportCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "teleport",
		Description: "Teleport the current session to remote runtime",
		Usage:       "/teleport",
		Hidden:      true,
	}
}

// Execute handles the /teleport command, routing to the appropriate teleport
// operation based on arguments. When FlagTeleport is disabled or the teleport
// service is not initialized, returns the fallback message.
func (c TeleportCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	if !featureflag.IsEnabled(featureflag.FlagTeleport) || c.Service == nil {
		return command.Result{
			Output: "Teleport command is not available in Claude Code Go yet. Remote handoff and teleport session flows remain unmigrated.",
		}, nil
	}

	raw := strings.TrimSpace(args.RawLine)

	// No arguments: create a new remote session
	if raw == "" {
		logger.DebugCF("commands", "creating new remote session via /teleport", nil)

		result, err := c.Service.TeleportToRemote(ctx, teleport.TeleportToRemoteOptions{
			Description: "Interactive session",
		})
		if err != nil {
			return command.Result{}, fmt.Errorf("teleport: %w", err)
		}

		output := fmt.Sprintf("Remote session created: %s\nTitle: %s", result.ID, result.Title)
		return command.Result{Output: output}, nil
	}

	// Argument provided: resume a remote session by ID
	sessionID := raw
	logger.DebugCF("commands", "resuming remote session", map[string]any{
		"session_id": sessionID,
	})

	messages, branch, err := c.Service.TeleportResumeCodeSession(ctx, sessionID)
	if err != nil {
		return command.Result{}, fmt.Errorf("teleport: %w", err)
	}

	_ = messages // Messages processed by caller during resume
	if branch != "" {
		return command.Result{
			Output: fmt.Sprintf("Session %s resumed. Branch: %s", sessionID, branch),
		}, nil
	}

	return command.Result{
		Output: fmt.Sprintf("Session %s resumed.", sessionID),
	}, nil
}
