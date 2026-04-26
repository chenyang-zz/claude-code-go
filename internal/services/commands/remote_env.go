package commands

import (
	"context"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const remoteEnvCommandFallback = "Remote environment configuration is not available in Claude Code Go yet. Teleport remote environment selection, policy-gated controls, and interactive remote env management remain unmigrated."

// RemoteEnvCommand exposes the minimum text-only /remote-env behavior before remote environment management exists in the Go runtime.
type RemoteEnvCommand struct{}

// Metadata returns the canonical slash descriptor for /remote-env.
func (c RemoteEnvCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "remote-env",
		Description: "Configure the default remote environment for teleport sessions",
		Usage:       "/remote-env",
	}
}

// Execute reports the stable /remote-env fallback supported by the current Go host.
func (c RemoteEnvCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx
	_ = args

	logger.DebugCF("commands", "rendered remote-env command fallback output", map[string]any{
		"remote_env_config_available": false,
	})

	return command.Result{
		Output: remoteEnvCommandFallback,
	}, nil
}
