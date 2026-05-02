package engine

import (
	"context"
	"sync"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// PostSamplingHook is called after the model stream completes and the assistant
// message has been appended to history, but before any tool calls are executed.
// This is the exact window where prompt suggestions can be generated based on
// the freshly produced assistant response.
type PostSamplingHook func(ctx context.Context, assistantMessage message.Message, history []message.Message, workingDir string) error

var (
	postSamplingHooksMu sync.Mutex
	postSamplingHooks   []PostSamplingHook
)

// RegisterPostSamplingHook registers a hook to be called after each model
// sampling completes but before tool execution begins.
func RegisterPostSamplingHook(hook PostSamplingHook) {
	postSamplingHooksMu.Lock()
	defer postSamplingHooksMu.Unlock()
	postSamplingHooks = append(postSamplingHooks, hook)
}

// ClearPostSamplingHooks removes all registered post-sampling hooks.
func ClearPostSamplingHooks() {
	postSamplingHooksMu.Lock()
	defer postSamplingHooksMu.Unlock()
	postSamplingHooks = nil
}

// firePostSamplingHooks executes all registered post-sampling hooks sequentially.
// Each hook runs with a 60-second timeout to avoid blocking the engine.
func (e *Runtime) firePostSamplingHooks(ctx context.Context, cwd string, assistantMessage message.Message, history []message.Message) {
	postSamplingHooksMu.Lock()
	hooks := make([]PostSamplingHook, len(postSamplingHooks))
	copy(hooks, postSamplingHooks)
	postSamplingHooksMu.Unlock()

	workingDir := e.workingDir(cwd)
	for _, hook := range hooks {
		if hook == nil {
			continue
		}
		hookCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		if err := hook(hookCtx, assistantMessage, history, workingDir); err != nil {
			logger.WarnCF("engine", "post-sampling hook error", map[string]any{
				"error": err.Error(),
			})
		}
		cancel()
	}
}
