package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/google/uuid"
	coresession "github.com/sheepzhao/claude-code-go/internal/core/session"
	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// asyncLaunchedOutput stores the minimum async launch metadata returned by AgentTool.
type asyncLaunchedOutput struct {
	// Status identifies this result as an asynchronously launched agent task.
	Status string `json:"status"`
	// AgentID stores the runtime task identifier.
	AgentID string `json:"agentId"`
	// Description stores the caller-provided task summary.
	Description string `json:"description"`
	// Prompt stores the original task prompt.
	Prompt string `json:"prompt"`
}

// backgroundTaskStopper implements best-effort cancellation for one background agent task.
type backgroundTaskStopper struct {
	// cancel stops the background task context.
	cancel context.CancelFunc
	// once guarantees cancellation runs at most once.
	once sync.Once
}

// Stop cancels one running background agent context.
func (s *backgroundTaskStopper) Stop() error {
	if s == nil || s.cancel == nil {
		return nil
	}
	s.once.Do(func() {
		s.cancel()
	})
	return nil
}

// launchBackground starts one asynchronous agent run and registers it in the shared background task store.
func (t *Tool) launchBackground(parentCtx context.Context, input Input) coretool.Result {
	taskID := "agent-" + uuid.NewString()
	summary := buildTaskSummary(input)

	runCtx, cancel := context.WithCancel(context.Background())
	if parentCtx != nil {
		runCtx, cancel = context.WithCancel(parentCtx)
	}
	stopper := &backgroundTaskStopper{cancel: cancel}

	if t.taskStore != nil {
		t.taskStore.Register(coresession.BackgroundTaskSnapshot{
			ID:                taskID,
			Type:              "agent",
			Status:            coresession.BackgroundTaskStatusRunning,
			Summary:           summary,
			ControlsAvailable: true,
		}, stopper)
	}

	go t.runBackgroundTask(runCtx, taskID, input)

	payload := asyncLaunchedOutput{
		Status:      "async_launched",
		AgentID:     taskID,
		Description: input.Description,
		Prompt:      input.Prompt,
	}
	resultJSON, err := json.Marshal(payload)
	if err != nil {
		return coretool.Result{Error: fmt.Sprintf("failed to marshal async launch output: %v", err)}
	}
	return coretool.Result{Output: string(resultJSON)}
}

// runBackgroundTask executes one asynchronous agent run and updates the shared task lifecycle state.
func (t *Tool) runBackgroundTask(ctx context.Context, taskID string, input Input) {
	summary := buildTaskSummary(input)
	if _, err := t.runnerFactory().Run(ctx, input); err != nil {
		status := coresession.BackgroundTaskStatusFailed
		if ctx != nil && ctx.Err() != nil {
			status = coresession.BackgroundTaskStatusStopped
		}
		logger.WarnCF("agent.tool", "background agent run failed", map[string]any{
			"task_id":       taskID,
			"subagent_type": input.SubagentType,
			"status":        status,
			"error":         err.Error(),
		})
		if t.taskStore != nil {
			t.taskStore.Update(coresession.BackgroundTaskSnapshot{
				ID:                taskID,
				Type:              "agent",
				Status:            status,
				Summary:           summary,
				ControlsAvailable: true,
			})
		}
		return
	}

	if t.taskStore != nil {
		t.taskStore.Update(coresession.BackgroundTaskSnapshot{
			ID:                taskID,
			Type:              "agent",
			Status:            coresession.BackgroundTaskStatusCompleted,
			Summary:           summary,
			ControlsAvailable: true,
		})
	}
	logger.DebugCF("agent.tool", "background agent run completed", map[string]any{
		"task_id":       taskID,
		"subagent_type": input.SubagentType,
	})
}

// buildTaskSummary returns one stable summary for background task listings.
func buildTaskSummary(input Input) string {
	summary := strings.TrimSpace(input.Description)
	if summary == "" {
		summary = strings.TrimSpace(input.Prompt)
	}
	if summary == "" {
		return "background agent task"
	}
	return summary
}
