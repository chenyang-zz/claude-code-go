package executor

import (
	"context"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/app/wiring"
	"github.com/sheepzhao/claude-code-go/internal/core/tool"
)

// stubTool is a minimal test double used to verify end-to-end tool dispatch.
type stubTool struct{}

// Name returns the registration name used by the executor test.
func (stubTool) Name() string { return "stub" }

// Description returns a short description for the test tool.
func (stubTool) Description() string { return "test stub tool" }

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
