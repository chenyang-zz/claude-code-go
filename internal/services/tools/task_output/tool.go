package task_output

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	coresession "github.com/sheepzhao/claude-code-go/internal/core/session"
	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	runtimesession "github.com/sheepzhao/claude-code-go/internal/runtime/session"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const Name = "TaskOutput"

// legacyAliases expose the old names "AgentOutputTool" and "BashOutputTool".
var legacyAliases = []string{"AgentOutputTool", "BashOutputTool"}

// TaskStore describes the minimal lifecycle store needed to read task output.
type TaskStore interface {
	Get(id string) (coresession.BackgroundTaskSnapshot, bool)
}

// Tool implements the deprecated TaskOutput tool. Models should prefer
// using the Read tool on the task output file path instead.
type Tool struct {
	taskStore *runtimesession.BackgroundTaskStore
}

// NewTool creates a TaskOutput tool that reads from the given task store.
func NewTool(taskStore *runtimesession.BackgroundTaskStore) *Tool {
	return &Tool{taskStore: taskStore}
}

func (t *Tool) Name() string { return Name }

func (t *Tool) Description() string {
	return "[Deprecated] — prefer Read on the task output file path"
}

func (t *Tool) Aliases() []string {
	return legacyAliases
}

// taskOutputInput defines the input schema for the TaskOutput tool.
type taskOutputInput struct {
	// TaskID identifies the background task to read output from.
	TaskID string `json:"task_id"`
	// Block controls whether to wait for task completion (default true, nil = true).
	Block *bool `json:"block,omitempty"`
	// Timeout is the maximum wait time in milliseconds (default 30000, max 600000).
	Timeout int `json:"timeout,omitempty"`
}

// taskOutputResult mirrors the TS retrieval_status + task shape.
type taskOutputResult struct {
	RetrievalStatus string         `json:"retrieval_status"`
	Task            *taskOutputData `json:"task"`
}

// taskOutputData holds the unified task output information.
type taskOutputData struct {
	TaskID      string `json:"task_id"`
	TaskType    string `json:"task_type"`
	Status      string `json:"status"`
	Description string `json:"description"`
	Output      string `json:"output,omitempty"`
	ExitCode    *int   `json:"exitCode,omitempty"`
	Error       string `json:"error,omitempty"`
}

func (t *Tool) InputSchema() coretool.InputSchema {
	return coretool.InputSchema{
		Properties: map[string]coretool.FieldSchema{
			"task_id": {
				Type:        coretool.ValueKindString,
				Description: "The task ID to get output from",
				Required:    true,
			},
			"block": {
				Type:        coretool.ValueKindBoolean,
				Description: "Whether to wait for completion",
			},
			"timeout": {
				Type:        coretool.ValueKindNumber,
				Description: "Max wait time in ms",
			},
		},
	}
}

func (t *Tool) IsReadOnly() bool { return true }

func (t *Tool) IsConcurrencySafe() bool { return true }

// applyDefaults fills in default values matching TS behavior.
func applyDefaults(input *taskOutputInput) {
	if input.Block == nil {
		defaultBlock := true
		input.Block = &defaultBlock
	}
	if input.Timeout <= 0 {
		input.Timeout = 30000
	}
	if input.Timeout > 600000 {
		input.Timeout = 600000
	}
}

// Invoke reads task output, optionally blocking until the task completes or times out.
func (t *Tool) Invoke(ctx context.Context, call coretool.Call) (coretool.Result, error) {
	if t == nil {
		return coretool.Result{Error: "TaskOutput tool: nil receiver"}, nil
	}

	input, err := coretool.DecodeInput[taskOutputInput](t.InputSchema(), call.Input)
	if err != nil {
		return coretool.Result{Error: fmt.Sprintf("invalid input: %v", err)}, nil
	}

	applyDefaults(&input)

	if strings.TrimSpace(input.TaskID) == "" {
		return coretool.Result{Error: "Task ID is required"}, nil
	}

	if t.taskStore == nil {
		return coretool.Result{Error: "TaskOutput tool: task store not configured"}, nil
	}

	snapshot, found := t.taskStore.Get(input.TaskID)
	if !found {
		return coretool.Result{Error: fmt.Sprintf("No task found with ID: %s", input.TaskID)}, nil
	}

	// Non-blocking: return current state immediately.
	if input.Block != nil && !*input.Block {
		return buildResult(snapshot)
	}

	// Blocking: poll until the task reaches a terminal state or timeout.
	return t.waitForCompletion(ctx, input.TaskID, input.Timeout)
}

// waitForCompletion polls the task store every 100ms until the task completes or times out.
func (t *Tool) waitForCompletion(ctx context.Context, taskID string, timeoutMs int) (coretool.Result, error) {
	deadline := time.Now().Add(time.Duration(timeoutMs) * time.Millisecond)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return coretool.Result{Error: "aborted"}, nil
		case <-ticker.C:
			snapshot, found := t.taskStore.Get(taskID)
			if !found {
				return coretool.Result{Error: fmt.Sprintf("No task found with ID: %s", taskID)}, nil
			}

			terminal := snapshot.Status != coresession.BackgroundTaskStatusRunning &&
				snapshot.Status != coresession.BackgroundTaskStatusPending

			if terminal || time.Now().After(deadline) {
				result, err := buildResult(snapshot)
				if err != nil {
					return result, err
				}

				// Label timeout when the task is still running after the deadline.
				if !terminal && time.Now().After(deadline) {
					resultData := taskOutputResult{
						RetrievalStatus: "timeout",
						Task:            buildTaskOutputData(snapshot),
					}
					data, _ := json.Marshal(resultData)
					return coretool.Result{Output: string(data)}, nil
				}

				return result, nil
			}
		}
	}
}

// buildResult constructs the final tool result from a task snapshot.
func buildResult(snapshot coresession.BackgroundTaskSnapshot) (coretool.Result, error) {
	retrievalStatus := "not_ready"
	terminal := snapshot.Status != coresession.BackgroundTaskStatusRunning &&
		snapshot.Status != coresession.BackgroundTaskStatusPending
	if terminal {
		retrievalStatus = "success"
	}

	resultData := taskOutputResult{
		RetrievalStatus: retrievalStatus,
		Task:            buildTaskOutputData(snapshot),
	}

	data, _ := json.Marshal(resultData)

	logger.DebugCF("task_output", "read task output", map[string]any{
		"task_id":          snapshot.ID,
		"type":             snapshot.Type,
		"status":           string(snapshot.Status),
		"retrieval_status": retrievalStatus,
	})

	return coretool.Result{Output: string(data)}, nil
}

// buildTaskOutputData converts a BackgroundTaskSnapshot into the structured task output.
func buildTaskOutputData(snapshot coresession.BackgroundTaskSnapshot) *taskOutputData {
	return &taskOutputData{
		TaskID:      snapshot.ID,
		TaskType:    snapshot.Type,
		Status:      string(snapshot.Status),
		Description: snapshot.Summary,
	}
}
