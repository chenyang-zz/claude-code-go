package awaysummary

import (
	"context"

	"github.com/sheepzhao/claude-code-go/internal/core/featureflag"
	"github.com/sheepzhao/claude-code-go/internal/core/message"
	"github.com/sheepzhao/claude-code-go/internal/core/model"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// PostTurnHookFunc matches the engine.PostTurnHook signature for registration.
type PostTurnHookFunc func(ctx context.Context, messages []message.Message, workingDir string) error

// currentSystem holds the active AwaySummary system after initialization.
var currentSystem *System

// IsAwaySummaryEnabled reports whether the away summary feature is enabled
// via the AWAY_SUMMARY feature flag.
func IsAwaySummaryEnabled() bool {
	return featureflag.IsEnabled(featureflag.FlagAwaySummary)
}

// IsInitialized reports whether the away summary system has been initialized.
func IsInitialized() bool {
	return currentSystem != nil
}

// InitAwaySummary initializes the away summary system. It registers a
// PostTurnHook that records activity timestamps after each turn. The actual
// generation is triggered by CheckAndGenerate, which should be called from
// the REPL runner before each engine turn.
//
// The client may be nil if the model client is not yet available; call
// SetModelClient before the first turn. The memBaseDir is the auto-memory
// directory root.
func InitAwaySummary(client model.Client, registerPostTurnHook func(hook PostTurnHookFunc), memBaseDir string, cfg Config) *System {
	if !IsAwaySummaryEnabled() {
		logger.DebugCF("awaysummary", "skipping init: away summary disabled", nil)
		return nil
	}

	sys := NewSystem(client, memBaseDir, cfg)
	currentSystem = sys

	// Register PostTurnHook that records the end-of-turn timestamp for idle detection.
	registerPostTurnHook(PostTurnHookFunc(func(ctx context.Context, messages []message.Message, workingDir string) error {
		sys.RecordActivity()
		return nil
	}))

	logger.DebugCF("awaysummary", "away summary initialized", map[string]any{
		"idle_threshold": cfg.IdleThreshold.String(),
		"model":          cfg.Model,
	})

	return sys
}

// CheckAndGenerate checks whether the user has been idle long enough to
// warrant an away summary. If so, it generates one and returns it as a
// system message to inject into the conversation. Returns nil if no
// summary is needed.
//
// This should be called from the REPL runner before each engine turn.
func CheckAndGenerate(ctx context.Context, messages []message.Message) *message.Message {
	if currentSystem == nil {
		return nil
	}

	if !currentSystem.ShouldGenerate() {
		return nil
	}

	text, err := currentSystem.Generate(ctx, messages)
	if err != nil {
		logger.WarnCF("awaysummary", "generate failed", map[string]any{
			"error": err.Error(),
		})
		return nil
	}

	if text == "" {
		return nil
	}

	logger.DebugCF("awaysummary", "generated away summary", map[string]any{
		"length": len(text),
	})

	return &message.Message{
		Role: message.RoleSystem,
		Content: []message.ContentPart{
			message.TextPart(text),
		},
	}
}
