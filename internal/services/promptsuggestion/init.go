package promptsuggestion

import (
	"context"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// PostSamplingHookFunc is the adapter type used by Init to register with the
// engine's post-sampling hook system.
type PostSamplingHookFunc func(ctx context.Context, assistantMessage message.Message, history []message.Message, workingDir string) error

// Init initializes the PromptSuggestion system.
//
// If prompt suggestion is not enabled (feature flag + env check), it returns
// nil and does not register any hook.
//
// The runner may be nil; in that case the suggester returns placeholder
// suggestions without forking a real agent.
//
// The returned cleanup function should be called on shutdown to unregister
// the post-sampling hook.
func Init(
	runner SubagentRunner,
	registerPostSamplingHook func(hook PostSamplingHookFunc),
	projectRoot string,
) (*Suggester, func()) {
	if !IsPromptSuggestionEnabled() {
		logger.DebugCF("promptsuggestion", "prompt suggestion disabled, skipping init", nil)
		return nil, func() {}
	}

	suggester := NewSuggester(runner)
	speculator := NewSpeculator()

	logger.DebugCF("promptsuggestion", "initializing prompt suggestion system", map[string]any{
		"project_root": projectRoot,
	})

	registerPostSamplingHook(func(ctx context.Context, assistantMessage message.Message, history []message.Message, workingDir string) error {
		// Generate suggestion based on the current turn
		outcome := suggester.TryGenerate(ctx, history)
		if outcome.Suppress != "" {
			logger.DebugCF("promptsuggestion", "suggestion suppressed", map[string]any{
				"reason": string(outcome.Suppress),
			})
			return nil
		}
		if outcome.Error != nil {
			logger.WarnCF("promptsuggestion", "suggestion generation error", map[string]any{
				"error": outcome.Error.Error(),
			})
			return nil
		}
		if outcome.Suggestion != nil && outcome.Suggestion.Text != "" {
			logger.DebugCF("promptsuggestion", "suggestion generated", map[string]any{
				"text":      outcome.Suggestion.Text,
				"prompt_id": string(outcome.Suggestion.PromptID),
			})

			// Start speculation if enabled
			if IsSpeculationEnabled() {
				speculator.Start(ctx, StartParams{
					SuggestionText: outcome.Suggestion.Text,
					Messages:       history,
				})
			}
		}
		return nil
	})

	cleanup := func() {
		suggester.Abort()
		speculator.Abort()
	}

	return suggester, cleanup
}
