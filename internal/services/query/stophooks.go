package query

import (
	"context"
	"fmt"

	"github.com/sheepzhao/claude-code-go/internal/core/hook"
	"github.com/sheepzhao/claude-code-go/internal/core/message"
)

// StopHookResult carries the outcome of stop hook execution.
type StopHookResult struct {
	BlockingErrors      []message.Message
	PreventContinuation bool
}

// HandleStopHooks runs stop hooks and returns blocking errors.
// In Go, this delegates to the existing runtime HookRunner infrastructure
// which already handles Stop, TeammateIdle, and TaskCompleted hooks.
func HandleStopHooks(
	ctx context.Context,
	hooksConfig hook.HooksConfig,
	hookRunner interface {
		RunStopHooks(ctx context.Context, config hook.HooksConfig, event hook.HookEvent, input any, cwd string) []hook.HookResult
	},
	messages []message.Message,
	cwd string,
) StopHookResult {
	if hookRunner == nil {
		return StopHookResult{}
	}

	results := hookRunner.RunStopHooks(ctx, hooksConfig, hook.HookEvent("Stop"), map[string]any{
		"messages": messages,
	}, cwd)

	var blockingErrors []message.Message
	preventContinuation := false

	for _, result := range results {
		if result.IsBlocking() {
			blockingErrors = append(blockingErrors, message.Message{
				Role: "user",
				Content: []message.ContentPart{{
					Type: "text",
					Text: result.Stderr,
				}},
			})
		}
		if result.PreventContinuation {
			preventContinuation = true
		}
	}

	return StopHookResult{
		BlockingErrors:      blockingErrors,
		PreventContinuation: preventContinuation,
	}
}

// Ensure the fmt import is used for potential future error formatting
var _ = fmt.Sprintf
