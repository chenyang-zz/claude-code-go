package commands

import (
	"context"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const remoteControlCommandFallback = "Remote-control bridge sessions are not available in Claude Code Go yet. Bridge connection, auth, and reconnect flows remain unmigrated."

// RemoteControlCommand exposes the minimum text-only /remote-control behavior before bridge mode exists in the Go host.
type RemoteControlCommand struct{}

// Metadata returns the canonical slash descriptor for /remote-control.
func (c RemoteControlCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "remote-control",
		Aliases:     []string{"rc"},
		Description: "Connect this terminal for remote-control sessions",
		Usage:       "/remote-control [name]",
	}
}

// Execute accepts optional arguments and reports the stable /remote-control fallback.
func (c RemoteControlCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx

	raw := strings.TrimSpace(args.RawLine)

	logger.DebugCF("commands", "rendered remote-control command fallback output", map[string]any{
		"remote_control_available": false,
		"arg_provided":             raw != "",
	})

	return command.Result{
		Output: remoteControlCommandFallback,
	}, nil
}
