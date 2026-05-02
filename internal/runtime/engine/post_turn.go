package engine

import (
	"context"
	"sync"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// PostTurnHook is called after each complete conversation turn (all tool calls finished).
// ctx is a context with a 60s timeout; messages is the full conversation history;
// workingDir is the current working directory.
// Return an error to log but not to surface to the user.
type PostTurnHook func(ctx context.Context, messages []message.Message, workingDir string) error

var (
	postTurnHooksMu sync.Mutex
	postTurnHooks   []PostTurnHook
)

// RegisterPostTurnHook registers a hook to be called after each complete turn.
func RegisterPostTurnHook(hook PostTurnHook) {
	postTurnHooksMu.Lock()
	defer postTurnHooksMu.Unlock()
	postTurnHooks = append(postTurnHooks, hook)
}

// ClearPostTurnHooks removes all registered post-turn hooks.
func ClearPostTurnHooks() {
	postTurnHooksMu.Lock()
	defer postTurnHooksMu.Unlock()
	postTurnHooks = nil
}

// firePostTurnHooks executes all registered post-turn hooks sequentially.
// Each hook runs with a 60-second timeout to avoid blocking the engine.
func (e *Runtime) firePostTurnHooks(ctx context.Context, cwd string, messages []message.Message) {
	postTurnHooksMu.Lock()
	hooks := make([]PostTurnHook, len(postTurnHooks))
	copy(hooks, postTurnHooks)
	postTurnHooksMu.Unlock()

	workingDir := e.workingDir(cwd)
	for _, hook := range hooks {
		if hook == nil {
			continue
		}
		hookCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		if err := hook(hookCtx, messages, workingDir); err != nil {
			logger.WarnCF("engine", "post-turn hook error", map[string]any{
				"error": err.Error(),
			})
		}
		cancel()
	}
}

// hasPendingToolCalls checks whether the latest assistant message
// in the conversation contains any tool_use content blocks that have
// not yet been executed. This mirrors the TS-side
// hasToolCallsInLastAssistantTurn semantics.
func hasPendingToolCalls(messages []message.Message) bool {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == message.RoleAssistant {
			return hasToolCallContent(messages[i].Content)
		}
	}
	return false
}

// hasToolCallContent reports whether any content block is a tool_use.
func hasToolCallContent(blocks []message.ContentPart) bool {
	for _, b := range blocks {
		if b.Type == "tool_use" {
			return true
		}
	}
	return false
}
