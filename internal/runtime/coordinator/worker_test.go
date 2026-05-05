package coordinator

import (
	"context"
	"testing"
)

func TestNewWorker(t *testing.T) {
	input := AgentInput{
		Description:  "test task",
		Prompt:       "do something",
		SubagentType: "worker",
	}

	w := NewWorker(input, nil)
	if w == nil {
		t.Fatal("expected non-nil worker")
	}
	if w.ID == "" {
		t.Error("expected non-empty ID")
	}
	if w.State != WorkerStateCreated {
		t.Errorf("expected state %s, got %s", WorkerStateCreated, w.State)
	}
	if w.Input.Description != "test task" {
		t.Errorf("expected description 'test task', got '%s'", w.Input.Description)
	}
	if w.Runner != nil {
		t.Error("expected nil runner")
	}
}

func TestWorkerStateString(t *testing.T) {
	tests := []struct {
		state    WorkerState
		expected string
	}{
		{WorkerStateCreated, "created"},
		{WorkerStateRunning, "running"},
		{WorkerStateCompleted, "completed"},
		{WorkerStateFailed, "failed"},
		{WorkerStateStopped, "stopped"},
		{WorkerState(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.state.String(); got != tt.expected {
			t.Errorf("WorkerState(%d).String() = %s, want %s", tt.state, got, tt.expected)
		}
	}
}

func TestWorkerExecuteNoRunner(t *testing.T) {
	input := AgentInput{
		Description:  "test task",
		Prompt:       "do something",
		SubagentType: "worker",
	}

	w := NewWorker(input, nil)
	_, err := w.Execute(context.Background())
	if err == nil {
		t.Error("expected error for nil runner")
	}
	if w.State != WorkerStateFailed {
		t.Errorf("expected state %s, got %s", WorkerStateFailed, w.State)
	}
}

func TestWorkerExecuteInvalidState(t *testing.T) {
	input := AgentInput{
		Description:  "test task",
		Prompt:       "do something",
		SubagentType: "worker",
	}

	w := NewWorker(input, nil)
	w.State = WorkerStateRunning // Invalid state for execution

	_, err := w.Execute(context.Background())
	if err == nil {
		t.Error("expected error for invalid state")
	}
}

func TestWorkerStopNotRunning(t *testing.T) {
	input := AgentInput{
		Description:  "test task",
		Prompt:       "do something",
		SubagentType: "worker",
	}

	w := NewWorker(input, nil)
	err := w.Stop()
	if err == nil {
		t.Error("expected error for stopping non-running worker")
	}
}

func TestWorkerCleanup(t *testing.T) {
	input := AgentInput{
		Description:  "test task",
		Prompt:       "do something",
		SubagentType: "worker",
	}

	w := NewWorker(input, nil)
	err := w.Cleanup()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestWorkerDuration(t *testing.T) {
	input := AgentInput{
		Description:  "test task",
		Prompt:       "do something",
		SubagentType: "worker",
	}

	w := NewWorker(input, nil)

	// Before execution, duration should be 0
	if d := w.Duration(); d != 0 {
		t.Errorf("expected 0 duration, got %v", d)
	}
}
