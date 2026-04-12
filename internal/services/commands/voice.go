package commands

import (
	"context"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const voiceCommandFallback = "Voice mode is not available in Claude Code Go yet. Microphone capture, push-to-talk, recording dependency checks, speech-to-text language handling, and voice settings persistence remain unmigrated."

// VoiceCommand exposes the minimum text-only /voice behavior before audio and speech host integrations exist in the Go runtime.
type VoiceCommand struct{}

// Metadata returns the canonical slash descriptor for /voice.
func (c VoiceCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "voice",
		Description: "Toggle voice mode",
		Usage:       "/voice",
	}
}

// Execute reports the stable voice fallback supported by the current Go host.
func (c VoiceCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx
	_ = args

	logger.DebugCF("commands", "rendered voice command fallback output", map[string]any{
		"voice_mode_available": false,
		"microphone_supported": false,
	})

	return command.Result{
		Output: voiceCommandFallback,
	}, nil
}
