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

	// Check concurrent worker limit
	if s.Config.MaxConcurrentWorkers > 0 && len(s.workers) >= s.Config.MaxConcurrentWorkers {
		return nil, fmt.Errorf("max concurrent workers reached: %d", s.Config.MaxConcurrentWorkers)
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
