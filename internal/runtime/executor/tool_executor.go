package executor

import (
	"context"
	"fmt"
	"sync"

	"github.com/sheepzhao/claude-code-go/internal/core/tool"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// ToolExecutor resolves tool calls against a registry and invokes the matched implementation.
type ToolExecutor struct {
	// registry is the source of truth for executable tool instances.
	registry tool.Registry
	// mu guards shared read-state updates across tool invocations.
	mu sync.RWMutex
	// readState stores the latest successful read snapshots for later write guards.
	readState tool.ReadStateSnapshot
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

	call.Context.ReadState = e.buildInvocationReadState(call.Context.ReadState)

	logger.DebugCF("tool_executor", "executing tool call", map[string]any{
		"tool_name":        call.Name,
		"call_id":          call.ID,
		"source":           call.Source,
		"working_dir":      call.Context.WorkingDir,
		"read_state_files": len(call.Context.ReadState.Files),
	})

	result, err := target.Invoke(ctx, call)
	if err != nil {
		return result, err
	}

	e.applyReadStateUpdate(result.Meta)
	return result, nil
}

// buildInvocationReadState merges executor-maintained snapshots with any caller-supplied state.
func (e *ToolExecutor) buildInvocationReadState(inline tool.ReadStateSnapshot) tool.ReadStateSnapshot {
	if e == nil {
		return inline.Clone()
	}

	e.mu.RLock()
	merged := e.readState.Clone()
	e.mu.RUnlock()
	merged.Merge(inline)
	return merged
}

// applyReadStateUpdate persists the read-state delta emitted by a successful tool invocation.
func (e *ToolExecutor) applyReadStateUpdate(meta map[string]any) {
	if e == nil || len(meta) == 0 {
		return
	}

	update, ok := meta["read_state"].(tool.ReadStateSnapshot)
	if !ok || len(update.Files) == 0 {
		return
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	e.readState.Merge(update)

	logger.DebugCF("tool_executor", "updated executor read state", map[string]any{
		"updated_files": len(update.Files),
		"total_files":   len(e.readState.Files),
	})
}
