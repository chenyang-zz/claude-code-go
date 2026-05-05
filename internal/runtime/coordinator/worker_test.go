package coordinator

import (
	"context"
	"sync"
	"testing"
	"time"
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

// blockingRunner is a mock runner that blocks until the context is cancelled.
type blockingRunner struct {
	started chan struct{}
}

func (r *blockingRunner) Run(ctx context.Context, _ AgentInput) (AgentOutput, error) {
	close(r.started)
	<-ctx.Done()
	return AgentOutput{}, ctx.Err()
}

func TestWorkerStopDuringExecute(t *testing.T) {
	runner := &blockingRunner{started: make(chan struct{})}
	input := AgentInput{
		Description:  "blocking task",
		Prompt:       "block until stopped",
		SubagentType: "worker",
	}

	w := NewWorker(input, runner)

	var wg sync.WaitGroup
	wg.Add(1)
	var execErr error
	go func() {
		defer wg.Done()
		_, execErr = w.Execute(context.Background())
	}()

	// Wait for Execute to start running
	<-runner.started

	// Give a moment for Execute to be in the Running state
	time.Sleep(10 * time.Millisecond)

	// Stop the worker while it's executing
	stopErr := w.Stop()
	if stopErr != nil {
		t.Fatalf("unexpected error stopping worker: %v", stopErr)
	}

	wg.Wait()

	// After Stop + Execute completes, state should be Stopped (not Failed)
	w.mu.Lock()
	state := w.State
	w.mu.Unlock()

	if state != WorkerStateStopped {
		t.Errorf("expected state %s after stop, got %s (execErr: %v)", WorkerStateStopped, state, execErr)
	}
}
