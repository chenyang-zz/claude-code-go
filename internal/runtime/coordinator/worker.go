package coordinator

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// WorkerState represents the lifecycle state of a Worker.
type WorkerState int

const (
	// WorkerStateCreated indicates the worker has been created but not yet executed.
	WorkerStateCreated WorkerState = iota
	// WorkerStateRunning indicates the worker is currently executing.
	WorkerStateRunning
	// WorkerStateCompleted indicates the worker has completed execution successfully.
	WorkerStateCompleted
	// WorkerStateFailed indicates the worker has failed during execution.
	WorkerStateFailed
	// WorkerStateStopped indicates the worker was stopped before completion.
	WorkerStateStopped
)

// String returns a human-readable representation of the WorkerState.
func (s WorkerState) String() string {
	switch s {
	case WorkerStateCreated:
		return "created"
	case WorkerStateRunning:
		return "running"
	case WorkerStateCompleted:
		return "completed"
	case WorkerStateFailed:
		return "failed"
	case WorkerStateStopped:
		return "stopped"
	default:
		return "unknown"
	}
}

// Worker represents a single worker agent with its lifecycle state.
type Worker struct {
	// ID is the unique identifier for this worker.
	ID string
	// Input holds the worker's execution input.
	Input AgentInput
	// State is the current lifecycle state.
	State WorkerState
	// Output holds the execution result after completion.
	Output AgentOutput
	// Error holds the error if execution failed.
	Error error
	// CreatedAt is when the worker was created.
	CreatedAt time.Time
	// StartedAt is when the worker started executing.
	StartedAt time.Time
	// CompletedAt is when the worker completed execution.
	CompletedAt time.Time
	// Runner executes the agent task.
	Runner AgentRunner
	// cancelFunc is used to stop a running worker.
	cancelFunc context.CancelFunc
	// mu protects concurrent access to worker state.
	mu sync.Mutex
}

// NewWorker creates a new Worker with the given input and runner.
func NewWorker(input AgentInput, r AgentRunner) *Worker {
	return &Worker{
		ID:        fmt.Sprintf("worker-%s", uuid.NewString()[:8]),
		Input:     input,
		State:     WorkerStateCreated,
		Runner:    r,
		CreatedAt: time.Now(),
	}
}

// Execute runs the worker agent and returns the output.
// It transitions the worker through states: created → running → completed/failed.
func (w *Worker) Execute(ctx context.Context) (AgentOutput, error) {
	w.mu.Lock()
	if w.State != WorkerStateCreated {
		w.mu.Unlock()
		return AgentOutput{}, fmt.Errorf("worker %q cannot execute from state %s", w.ID, w.State)
	}
	w.State = WorkerStateRunning
	w.StartedAt = time.Now()
	w.mu.Unlock()

	if w.Runner == nil {
		w.mu.Lock()
		w.State = WorkerStateFailed
		w.Error = fmt.Errorf("worker runner is nil")
		w.CompletedAt = time.Now()
		w.mu.Unlock()
		return AgentOutput{}, w.Error
	}

	// Create cancellable context for this worker
	ctx, cancel := context.WithCancel(ctx)
	w.mu.Lock()
	w.cancelFunc = cancel
	w.mu.Unlock()
	defer cancel()

	logger.DebugCF("coordinator.worker", "worker executing", map[string]any{
		"worker_id":     w.ID,
		"subagent_type": w.Input.SubagentType,
	})

	// Execute the agent task
	output, err := w.Runner.Run(ctx, w.Input)

	w.mu.Lock()
	defer w.mu.Unlock()

	// If the worker was stopped (via Stop()), preserve that state
	if w.State == WorkerStateStopped {
		return output, fmt.Errorf("worker %q was stopped", w.ID)
	}

	if err != nil {
		w.State = WorkerStateFailed
		w.Error = err
		w.CompletedAt = time.Now()

		logger.WarnCF("coordinator.worker", "worker execution failed", map[string]any{
			"worker_id": w.ID,
			"error":     err.Error(),
		})

		return output, err
	}

	w.State = WorkerStateCompleted
	w.Output = output
	w.CompletedAt = time.Now()

	logger.DebugCF("coordinator.worker", "worker completed", map[string]any{
		"worker_id":      w.ID,
		"duration_ms":    output.TotalDurationMs,
		"total_tokens":   output.TotalTokens,
		"tool_use_count": output.TotalToolUseCount,
	})

	return output, nil
}

// Stop stops a running worker by cancelling its context.
func (w *Worker) Stop() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.State != WorkerStateRunning {
		return fmt.Errorf("worker %q cannot stop from state %s", w.ID, w.State)
	}

	if w.cancelFunc != nil {
		w.cancelFunc()
	}

	w.State = WorkerStateStopped
	w.CompletedAt = time.Now()

	logger.DebugCF("coordinator.worker", "worker stopped", map[string]any{
		"worker_id": w.ID,
	})

	return nil
}

// Cleanup releases resources held by the worker.
// This is a no-op for the minimal implementation.
func (w *Worker) Cleanup() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Clean up any resources if needed
	// For now, this is a no-op as Runner handles its own cleanup

	logger.DebugCF("coordinator.worker", "worker cleanup", map[string]any{
		"worker_id": w.ID,
		"state":     w.State.String(),
	})

	return nil
}

// Duration returns the total duration of the worker's execution.
func (w *Worker) Duration() time.Duration {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.StartedAt.IsZero() {
		return 0
	}
	if w.CompletedAt.IsZero() {
		return time.Since(w.StartedAt)
	}
	return w.CompletedAt.Sub(w.StartedAt)
}
