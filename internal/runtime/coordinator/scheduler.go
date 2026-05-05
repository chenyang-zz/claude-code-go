package coordinator

import (
	"context"
	"fmt"
	"sync"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// Scheduler manages the lifecycle of worker agents in coordinator mode.
// It provides methods to create, execute, and clean up workers.
type Scheduler struct {
	// Runner executes agent tasks.
	Runner AgentRunner
	// Config holds scheduler configuration options.
	Config SchedulerConfig
	// mu protects concurrent access to workers map.
	mu sync.Mutex
	// workers tracks active workers by their IDs.
	workers map[string]*Worker
}

// SchedulerConfig holds configuration options for the Scheduler.
type SchedulerConfig struct {
	// MaxConcurrentWorkers limits the number of workers running simultaneously.
	// 0 means no limit.
	MaxConcurrentWorkers int
	// DefaultMaxTurns is the default max turns for workers if not specified.
	DefaultMaxTurns int
}

// DefaultSchedulerConfig returns a SchedulerConfig with sensible defaults.
func DefaultSchedulerConfig() SchedulerConfig {
	return SchedulerConfig{
		MaxConcurrentWorkers: 0, // no limit
		DefaultMaxTurns:      8,
	}
}

// NewScheduler creates a new Scheduler with the given runner and config.
func NewScheduler(r AgentRunner, cfg SchedulerConfig) *Scheduler {
	return &Scheduler{
		Runner:  r,
		Config:  cfg,
		workers: make(map[string]*Worker),
	}
}

// Schedule creates and executes a worker agent with the given input.
// It returns the worker output or an error.
func (s *Scheduler) Schedule(ctx context.Context, input AgentInput) (AgentOutput, error) {
	if s.Runner == nil {
		return AgentOutput{}, fmt.Errorf("scheduler runner is nil")
	}

	// Create worker
	w, err := s.CreateWorker(input)
	if err != nil {
		return AgentOutput{}, fmt.Errorf("failed to create worker: %w", err)
	}

	// Ensure worker is removed from tracking map after execution
	defer s.RemoveWorker(w.ID)

	// Execute worker
	output, err := w.Execute(ctx)
	if err != nil {
		logger.WarnCF("coordinator.scheduler", "worker execution failed", map[string]any{
			"worker_id": w.ID,
			"error":     err.Error(),
		})
	}

	// Cleanup worker
	if cleanupErr := w.Cleanup(); cleanupErr != nil {
		logger.WarnCF("coordinator.scheduler", "worker cleanup failed", map[string]any{
			"worker_id": w.ID,
			"error":     cleanupErr.Error(),
		})
	}

	return output, err
}

// CreateWorker creates a new Worker with the given input but does not execute it.
func (s *Scheduler) CreateWorker(input AgentInput) (*Worker, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check concurrent worker limit (count created + running workers to prevent
	// ScheduleAsync from bypassing the limit before Execute() flips state to running)
	if s.Config.MaxConcurrentWorkers > 0 {
		activeCount := 0
		for _, w := range s.workers {
			if w.State == WorkerStateCreated || w.State == WorkerStateRunning {
				activeCount++
			}
		}
		if activeCount >= s.Config.MaxConcurrentWorkers {
			return nil, fmt.Errorf("max concurrent workers reached: %d", s.Config.MaxConcurrentWorkers)
		}
	}

	// Create worker
	w := NewWorker(input, s.Runner)
	s.workers[w.ID] = w

	logger.DebugCF("coordinator.scheduler", "worker created", map[string]any{
		"worker_id":     w.ID,
		"subagent_type": input.SubagentType,
	})

	return w, nil
}

// GetWorker returns the worker with the given ID, or nil if not found.
func (s *Scheduler) GetWorker(id string) *Worker {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.workers[id]
}

// RemoveWorker removes a worker from the scheduler's tracking map.
func (s *Scheduler) RemoveWorker(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.workers, id)
}

// ActiveWorkers returns the number of active workers being tracked.
func (s *Scheduler) ActiveWorkers() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.workers)
}

// StopWorker stops a running worker by its ID.
func (s *Scheduler) StopWorker(id string) error {
	s.mu.Lock()
	w, ok := s.workers[id]
	s.mu.Unlock()

	if !ok {
		return fmt.Errorf("worker %q not found", id)
	}

	return w.Stop()
}

// WorkerResult holds the result of an async worker execution.
type WorkerResult struct {
	// Worker is the completed worker instance.
	Worker *Worker
	// Output holds the execution result.
	Output AgentOutput
	// Error holds the error if execution failed.
	Error error
}

// ScheduleAsync creates and starts a worker in a background goroutine.
// It returns the worker and a channel that receives the result when complete.
// The channel is buffered with size 1 and is closed after sending.
// The worker is removed from the tracking map after execution completes.
func (s *Scheduler) ScheduleAsync(ctx context.Context, input AgentInput) (*Worker, <-chan WorkerResult) {
	resultCh := make(chan WorkerResult, 1)

	if s.Runner == nil {
		resultCh <- WorkerResult{Error: fmt.Errorf("scheduler runner is nil")}
		close(resultCh)
		return nil, resultCh
	}

	w, err := s.CreateWorker(input)
	if err != nil {
		resultCh <- WorkerResult{Error: fmt.Errorf("failed to create worker: %w", err)}
		close(resultCh)
		return nil, resultCh
	}

	go func() {
		defer close(resultCh)
		defer s.RemoveWorker(w.ID)

		output, execErr := w.Execute(ctx)

		if execErr != nil {
			logger.WarnCF("coordinator.scheduler", "async worker execution failed", map[string]any{
				"worker_id": w.ID,
				"error":     execErr.Error(),
			})
		}

		if cleanupErr := w.Cleanup(); cleanupErr != nil {
			logger.WarnCF("coordinator.scheduler", "async worker cleanup failed", map[string]any{
				"worker_id": w.ID,
				"error":     cleanupErr.Error(),
			})
		}

		resultCh <- WorkerResult{Worker: w, Output: output, Error: execErr}
	}()

	logger.DebugCF("coordinator.scheduler", "async worker scheduled", map[string]any{
		"worker_id":     w.ID,
		"subagent_type": input.SubagentType,
	})

	return w, resultCh
}

// CollectResults reads from multiple async worker result channels and returns
// all results. It blocks until all channels are closed or the context is done.
// If the context is cancelled, remaining results are discarded.
func CollectResults(ctx context.Context, channels []<-chan WorkerResult) []WorkerResult {
	if len(channels) == 0 {
		return nil
	}

	results := make([]WorkerResult, 0, len(channels))
	merged := mergeChannels(channels)

	for {
		select {
		case <-ctx.Done():
			results = append(results, WorkerResult{Error: ctx.Err()})
			return results
		case result, ok := <-merged:
			if !ok {
				return results
			}
			results = append(results, result)
		}
	}
}

// mergeChannels combines multiple read-only channels into a single channel.
// Results are forwarded as they arrive from any input channel.
// The output channel is closed when all input channels are closed.
// The merged channel is buffered to len(channels) to prevent goroutine leaks
// when the consumer stops reading early (e.g., on context cancellation).
func mergeChannels(channels []<-chan WorkerResult) <-chan WorkerResult {
	merged := make(chan WorkerResult, len(channels))
	go func() {
		defer close(merged)
		var wg sync.WaitGroup
		for _, ch := range channels {
			wg.Add(1)
			go func(c <-chan WorkerResult) {
				defer wg.Done()
				for result := range c {
					merged <- result
				}
			}(ch)
		}
		wg.Wait()
	}()
	return merged
}
