package executor

import (
	"context"
	"testing"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/app/wiring"
	"github.com/sheepzhao/claude-code-go/internal/core/tool"
)

// stubTool is a minimal test double used to verify end-to-end tool dispatch.
type stubTool struct{}

// Name returns the registration name used by the executor test.
func (stubTool) Name() string { return "stub" }

// Description returns a short description for the test tool.
func (stubTool) Description() string { return "test stub tool" }

// InputSchema returns the synthetic schema used by the executor test.
func (stubTool) InputSchema() tool.InputSchema { return tool.InputSchema{} }

// IsReadOnly reports that the stub does not mutate state.
func (stubTool) IsReadOnly() bool { return true }

// IsConcurrencySafe reports that the stub can be invoked concurrently in tests.
func (stubTool) IsConcurrencySafe() bool { return true }

// Invoke returns a fixed payload and echoes selected call context for assertions.
func (stubTool) Invoke(_ context.Context, call tool.Call) (tool.Result, error) {
	return tool.Result{
		Output: "ok",
		Meta: map[string]any{
			"working_dir": call.Context.WorkingDir,
			"invoker":     call.Context.Invoker,
		},
	}, nil
}

// TestToolExecutorExecute verifies registry wiring and call context propagation through the executor.
func TestToolExecutorExecute(t *testing.T) {
	modules, err := wiring.NewModules(stubTool{})
	if err != nil {
		t.Fatalf("NewModules() error = %v", err)
	}

	executor := NewToolExecutor(modules.Tools)
	result, err := executor.Execute(context.Background(), tool.Call{
		ID:     "call-1",
		Name:   "stub",
		Source: "test",
		Context: tool.UseContext{
			WorkingDir: "/tmp/project",
			Invoker:    "unit-test",
		},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != "ok" {
		t.Fatalf("Execute() output = %q, want %q", result.Output, "ok")
	}

	if got := result.Meta["working_dir"]; got != "/tmp/project" {
		t.Fatalf("Execute() working_dir = %v, want %q", got, "/tmp/project")
	}
}

// readTrackingTool is a test double that emits one read-state delta after a synthetic read.
type readTrackingTool struct{}

// Name returns the stable registration name used by the read-state test.
func (readTrackingTool) Name() string { return "read" }

// Description returns a short summary for the read-state test tool.
func (readTrackingTool) Description() string { return "test read tool" }

// InputSchema returns the synthetic schema used by the read-state test.
func (readTrackingTool) InputSchema() tool.InputSchema { return tool.InputSchema{} }

// IsReadOnly reports that the synthetic read never mutates external state.
func (readTrackingTool) IsReadOnly() bool { return true }

// IsConcurrencySafe reports that the test read tool is safe to invoke concurrently.
func (readTrackingTool) IsConcurrencySafe() bool { return true }

// Invoke emits a read-state update to simulate one successful FileReadTool call.
func (readTrackingTool) Invoke(_ context.Context, _ tool.Call) (tool.Result, error) {
	return tool.Result{
		Output: "read-ok",
		Meta: map[string]any{
			"read_state": tool.ReadStateSnapshot{
				Files: map[string]tool.ReadState{
					"/tmp/project/main.go": {
						ReadAt:          time.Unix(100, 0),
						ObservedModTime: time.Unix(90, 0),
						IsPartial:       false,
					},
				},
			},
		},
	}, nil
}

// writeTrackingTool is a test double that exposes the invocation read state for assertions.
type writeTrackingTool struct{}

// Name returns the stable registration name used by the read-state test.
func (writeTrackingTool) Name() string { return "write" }

// Description returns a short summary for the write-state test tool.
func (writeTrackingTool) Description() string { return "test write tool" }

// InputSchema returns the synthetic schema used by the read/write-state test.
func (writeTrackingTool) InputSchema() tool.InputSchema { return tool.InputSchema{} }

// IsReadOnly reports that the test focuses on context propagation rather than mutation semantics.
func (writeTrackingTool) IsReadOnly() bool { return false }

// IsConcurrencySafe reports that the test write tool is safe to invoke concurrently.
func (writeTrackingTool) IsConcurrencySafe() bool { return true }

// Invoke returns the invocation read state so the executor test can inspect it.
func (writeTrackingTool) Invoke(_ context.Context, call tool.Call) (tool.Result, error) {
	return tool.Result{
		Output: "write-ok",
		Meta: map[string]any{
			"read_state_files": len(call.Context.ReadState.Files),
			"read_state":       call.Context.ReadState,
		},
	}, nil
}

// TestToolExecutorMaintainsReadState verifies that successful read calls update executor state for later tool invocations.
func TestToolExecutorMaintainsReadState(t *testing.T) {
	modules, err := wiring.NewModules(readTrackingTool{}, writeTrackingTool{})
	if err != nil {
		t.Fatalf("NewModules() error = %v", err)
	}

	executor := NewToolExecutor(modules.Tools)

	if _, err := executor.Execute(context.Background(), tool.Call{
		ID:   "call-read",
		Name: "read",
		Context: tool.UseContext{
			WorkingDir: "/tmp/project",
		},
	}); err != nil {
		t.Fatalf("Execute(read) error = %v", err)
	}

	result, err := executor.Execute(context.Background(), tool.Call{
		ID:   "call-write",
		Name: "write",
		Context: tool.UseContext{
			WorkingDir: "/tmp/project",
		},
	})
	if err != nil {
		t.Fatalf("Execute(write) error = %v", err)
	}

	snapshot, ok := result.Meta["read_state"].(tool.ReadStateSnapshot)
	if !ok {
		t.Fatalf("Execute(write) read_state type = %T", result.Meta["read_state"])
	}

	state, ok := snapshot.Lookup("/tmp/project/main.go")
	if !ok {
		t.Fatal("Execute(write) missing propagated read state for /tmp/project/main.go")
	}
	if state.ReadAt != time.Unix(100, 0) {
		t.Fatalf("Execute(write) ReadAt = %v, want %v", state.ReadAt, time.Unix(100, 0))
	}
	if state.ObservedModTime != time.Unix(90, 0) {
		t.Fatalf("Execute(write) ObservedModTime = %v, want %v", state.ObservedModTime, time.Unix(90, 0))
	}
	if state.IsPartial {
		t.Fatal("Execute(write) IsPartial = true, want false")
	}
}

// TestToolExecutorMergesInlineReadState verifies executor state is merged with caller-supplied snapshots before invocation.
func TestToolExecutorMergesInlineReadState(t *testing.T) {
	modules, err := wiring.NewModules(readTrackingTool{}, writeTrackingTool{})
	if err != nil {
		t.Fatalf("NewModules() error = %v", err)
	}

	executor := NewToolExecutor(modules.Tools)

	if _, err := executor.Execute(context.Background(), tool.Call{
		ID:   "call-read",
		Name: "read",
		Context: tool.UseContext{
			WorkingDir: "/tmp/project",
		},
	}); err != nil {
		t.Fatalf("Execute(read) error = %v", err)
	}

	inlineReadAt := time.Unix(200, 0)
	inlineModTime := time.Unix(180, 0)
	result, err := executor.Execute(context.Background(), tool.Call{
		ID:   "call-write",
		Name: "write",
		Context: tool.UseContext{
			WorkingDir: "/tmp/project",
			ReadState: tool.ReadStateSnapshot{
				Files: map[string]tool.ReadState{
					"/tmp/project/other.go": {
						ReadAt:          inlineReadAt,
						ObservedModTime: inlineModTime,
						IsPartial:       true,
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Execute(write) error = %v", err)
	}

	snapshot, ok := result.Meta["read_state"].(tool.ReadStateSnapshot)
	if !ok {
		t.Fatalf("Execute(write) read_state type = %T", result.Meta["read_state"])
	}
	if len(snapshot.Files) != 2 {
		t.Fatalf("Execute(write) len(read_state.Files) = %d, want 2", len(snapshot.Files))
	}

	trackedState, ok := snapshot.Lookup("/tmp/project/main.go")
	if !ok {
		t.Fatal("Execute(write) missing executor-tracked read state")
	}
	if trackedState.ReadAt != time.Unix(100, 0) {
		t.Fatalf("Execute(write) tracked ReadAt = %v, want %v", trackedState.ReadAt, time.Unix(100, 0))
	}

	inlineState, ok := snapshot.Lookup("/tmp/project/other.go")
	if !ok {
		t.Fatal("Execute(write) missing inline read state")
	}
	if inlineState.ReadAt != inlineReadAt {
		t.Fatalf("Execute(write) inline ReadAt = %v, want %v", inlineState.ReadAt, inlineReadAt)
	}
	if inlineState.ObservedModTime != inlineModTime {
		t.Fatalf("Execute(write) inline ObservedModTime = %v, want %v", inlineState.ObservedModTime, inlineModTime)
	}
	if !inlineState.IsPartial {
		t.Fatal("Execute(write) inline IsPartial = false, want true")
	}
}
