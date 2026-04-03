package executor

import (
	"context"
	"fmt"

	"github.com/sheepzhao/claude-code-go/internal/core/tool"
)

// ToolExecutor resolves tool calls against a registry and invokes the matched implementation.
type ToolExecutor struct {
	// registry is the source of truth for executable tool instances.
	registry tool.Registry
}

// NewToolExecutor builds an executor backed by the provided tool registry.
func NewToolExecutor(registry tool.Registry) *ToolExecutor {
	return &ToolExecutor{registry: registry}
}

// Execute validates the call envelope, resolves the target tool, and delegates invocation.
func (e *ToolExecutor) Execute(ctx context.Context, call tool.Call) (tool.Result, error) {
	if e == nil || e.registry == nil {
		return tool.Result{}, fmt.Errorf("tool executor: registry is not configured")
	}

	if call.Name == "" {
		return tool.Result{}, fmt.Errorf("tool executor: empty tool name")
	}

	// Fail before invocation when the name cannot be resolved so tools never receive an invalid dispatch.
	target, ok := e.registry.Get(call.Name)
	if !ok {
		return tool.Result{}, fmt.Errorf("tool executor: tool %q not found", call.Name)
	}

	return target.Invoke(ctx, call)
}
