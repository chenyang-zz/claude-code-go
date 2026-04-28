package agent

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/agent"
	coresession "github.com/sheepzhao/claude-code-go/internal/core/session"
	"github.com/sheepzhao/claude-code-go/internal/core/tool"
	"github.com/sheepzhao/claude-code-go/internal/runtime/engine"
	runtimesession "github.com/sheepzhao/claude-code-go/internal/runtime/session"
)

type fakeTeammateStarter struct {
	pid     int
	err     error
	lastReq teammateSpawnRequest
	invoked bool
}

func (f *fakeTeammateStarter) Start(_ context.Context, req teammateSpawnRequest) (int, error) {
	f.invoked = true
	f.lastReq = req
	if f.err != nil {
		return 0, f.err
	}
	return f.pid, nil
}

func TestTool_Name(t *testing.T) {
	tool := NewTool(nil, nil, nil, nil, nil)
	if got := tool.Name(); got != "Agent" {
		t.Errorf("Name() = %q, want %q", got, "Agent")
	}
}

func TestTool_Description(t *testing.T) {
	tool := NewTool(nil, nil, nil, nil, nil)
	got := tool.Description()
	if got == "" {
		t.Error("Description() should not be empty")
	}
	// Verify it contains the fallback content when registry is nil
	if !strings.Contains(got, "Launch a specialized agent") {
		t.Error("Description() missing expected content")
	}
}

func TestTool_Description_WithRegistry(t *testing.T) {
	registry := agent.NewInMemoryRegistry()
	_ = registry.Register(agent.Definition{
		AgentType: "explore",
		WhenToUse: "Search specialist",
		Tools:     []string{"Read", "Bash"},
	})
	tool := NewTool(registry, nil, nil, nil, nil)
	got := tool.Description()
	if got == "" {
		t.Error("Description() should not be empty")
	}
	// With registry, should contain dynamic content
	if !strings.Contains(got, "Available agent types") {
		t.Error("Description() missing dynamic content")
	}
	if !strings.Contains(got, "- explore: Search specialist (Tools: Read, Bash)") {
		t.Error("Description() missing agent listing")
	}
}

func TestTool_IsReadOnly(t *testing.T) {
	tool := NewTool(nil, nil, nil, nil, nil)
	if got := tool.IsReadOnly(); got != false {
		t.Errorf("IsReadOnly() = %v, want false", got)
	}
}

func TestTool_IsConcurrencySafe(t *testing.T) {
	tool := NewTool(nil, nil, nil, nil, nil)
	if got := tool.IsConcurrencySafe(); got != true {
		t.Errorf("IsConcurrencySafe() = %v, want true", got)
	}
}

func TestTool_InputSchema(t *testing.T) {
	tool := NewTool(nil, nil, nil, nil, nil)
	schema := tool.InputSchema()

	requiredFields := []string{"description", "prompt"}
	for _, field := range requiredFields {
		fs, ok := schema.Properties[field]
		if !ok {
			t.Errorf("InputSchema() missing required field %q", field)
			continue
		}
		if !fs.Required {
			t.Errorf("field %q should be required", field)
		}
	}

	optionalFields := []string{"subagent_type", "model", "run_in_background", "name", "team_name", "mode", "cwd"}
	for _, field := range optionalFields {
		fs, ok := schema.Properties[field]
		if !ok {
			t.Errorf("InputSchema() missing optional field %q", field)
			continue
		}
		if fs.Required {
			t.Errorf("field %q should not be required", field)
		}
	}
}

func TestTool_Invoke_TeammateSpawnSuccess(t *testing.T) {
	registry := agent.NewInMemoryRegistry()
	parentRuntime := &engine.Runtime{}
	parentRuntime.SessionConfig.HasSettingSourcesFlag = true
	parentRuntime.SessionConfig.SettingSourcesFlag = "project,local"
	agentTool := NewTool(registry, parentRuntime, nil, nil, nil)
	starter := &fakeTeammateStarter{pid: 4321}
	agentTool.teammateStarter = starter

	call := tool.Call{
		Input: map[string]any{
			"description": "spawn teammate",
			"prompt":      "work on tests",
			"name":        "tester",
			"team_name":   "alpha",
			"cwd":         "/tmp",
		},
	}
	result, err := agentTool.Invoke(context.Background(), call)
	if err != nil {
		t.Fatalf("Invoke() unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("Invoke() result.Error = %q, want empty", result.Error)
	}
	if !starter.invoked {
		t.Fatal("expected teammate starter to be invoked")
	}
	if starter.lastReq.Name != "tester" || starter.lastReq.TeamName != "alpha" || starter.lastReq.Cwd != "/tmp" {
		t.Fatalf("starter request = %+v, want name/team/cwd populated", starter.lastReq)
	}
	if !starter.lastReq.SessionConfig.HasSettingSourcesFlag || starter.lastReq.SessionConfig.SettingSourcesFlag != "project,local" {
		t.Fatalf("starter session config = %+v, want setting-sources pass-through fields", starter.lastReq.SessionConfig)
	}
	if !strings.Contains(result.Output, "\"agentId\":\"tester@alpha\"") {
		t.Fatalf("Invoke() output = %q, want teammate agent id", result.Output)
	}
}

func TestTool_Invoke_TeammateSpawnFailure(t *testing.T) {
	registry := agent.NewInMemoryRegistry()
	parentRuntime := &engine.Runtime{}
	agentTool := NewTool(registry, parentRuntime, nil, nil, nil)
	agentTool.teammateStarter = &fakeTeammateStarter{err: errors.New("spawn boom")}

	call := tool.Call{
		Input: map[string]any{
			"description": "spawn teammate",
			"prompt":      "work on tests",
			"name":        "tester",
			"team_name":   "alpha",
		},
	}
	result, err := agentTool.Invoke(context.Background(), call)
	if err != nil {
		t.Fatalf("Invoke() unexpected error: %v", err)
	}
	if !strings.Contains(result.Error, "teammate spawn failed") {
		t.Fatalf("Invoke() error = %q, want teammate spawn failed", result.Error)
	}
}

func TestTeammateSpawnCLIArgsIncludesSettingSourcesFlag(t *testing.T) {
	got := teammateSpawnCLIArgs(tool.SessionConfigSnapshot{
		HasSettingSourcesFlag: true,
		SettingSourcesFlag:    "project,local",
	})
	if len(got) != 2 || got[0] != "--setting-sources" || got[1] != "project,local" {
		t.Fatalf("teammateSpawnCLIArgs() = %#v, want [--setting-sources project,local]", got)
	}
}

func TestTeammateSpawnCLIArgsSkipsSettingSourcesWhenNotSet(t *testing.T) {
	got := teammateSpawnCLIArgs(tool.SessionConfigSnapshot{})
	if len(got) != 0 {
		t.Fatalf("teammateSpawnCLIArgs() = %#v, want empty", got)
	}
}

func TestTool_Invoke_NilRegistry(t *testing.T) {
	agentTool := NewTool(nil, nil, nil, nil, nil)
	call := tool.Call{
		Input: map[string]any{
			"description": "test task",
			"prompt":      "do something",
		},
	}
	result, err := agentTool.Invoke(context.Background(), call)
	if err != nil {
		t.Fatalf("Invoke() unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Error("expected error result when registry is nil")
	}
}

func TestTool_Invoke_NilParentRuntime(t *testing.T) {
	registry := agent.NewInMemoryRegistry()
	agentTool := NewTool(registry, nil, nil, nil, nil)
	call := tool.Call{
		Input: map[string]any{
			"description": "test task",
			"prompt":      "do something",
		},
	}
	result, err := agentTool.Invoke(context.Background(), call)
	if err != nil {
		t.Fatalf("Invoke() unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Error("expected error result when parent runtime is nil")
	}
}

func TestTool_Invoke_InvalidInput(t *testing.T) {
	// Create a minimal registry and a dummy runtime so we pass the nil checks.
	registry := agent.NewInMemoryRegistry()
	// We can't easily create a working engine.Runtime without a real client,
	// but for the invalid-input path the runtime is only checked for nil.
	// Use a zero-value runtime; the decode failure should happen before runner.Run.
	parentRuntime := &engine.Runtime{}
	agentTool := NewTool(registry, parentRuntime, nil, nil, nil)

	call := tool.Call{
		Input: map[string]any{
			// Missing required "description" and "prompt"
			"subagent_type": "Explore",
		},
	}
	result, err := agentTool.Invoke(context.Background(), call)
	if err != nil {
		t.Fatalf("Invoke() unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Error("expected error result for invalid input")
	}
}

type fakeBackgroundTaskStore struct {
	mu    sync.Mutex
	tasks map[string]coresession.BackgroundTaskSnapshot
}

func (f *fakeBackgroundTaskStore) Register(task coresession.BackgroundTaskSnapshot, _ interface{ Stop() error }) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.tasks == nil {
		f.tasks = map[string]coresession.BackgroundTaskSnapshot{}
	}
	f.tasks[task.ID] = task
}

func (f *fakeBackgroundTaskStore) Update(task coresession.BackgroundTaskSnapshot) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.tasks == nil {
		return false
	}
	if _, ok := f.tasks[task.ID]; !ok {
		return false
	}
	f.tasks[task.ID] = task
	return true
}

func (f *fakeBackgroundTaskStore) Remove(id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.tasks, id)
}

func (f *fakeBackgroundTaskStore) list() []coresession.BackgroundTaskSnapshot {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]coresession.BackgroundTaskSnapshot, 0, len(f.tasks))
	for _, task := range f.tasks {
		out = append(out, task)
	}
	return out
}

type blockingRunner struct {
	wait <-chan struct{}
}

func (b *blockingRunner) Run(_ context.Context, _ Input) (Output, error) {
	<-b.wait
	return Output{}, nil
}

type errorRunner struct {
	err error
}

func (e *errorRunner) Run(_ context.Context, _ Input) (Output, error) {
	return Output{}, e.err
}

type successRunner struct{}

func (s *successRunner) Run(_ context.Context, _ Input) (Output, error) {
	return Output{}, nil
}

type ctxBlockingRunner struct{}

func (c *ctxBlockingRunner) Run(ctx context.Context, _ Input) (Output, error) {
	<-ctx.Done()
	return Output{}, ctx.Err()
}

func TestTool_Invoke_RunInBackgroundRegistersTask(t *testing.T) {
	registry := agent.NewInMemoryRegistry()
	parentRuntime := &engine.Runtime{}
	taskStore := &fakeBackgroundTaskStore{}
	agentTool := NewTool(registry, parentRuntime, nil, nil, taskStore)

	wait := make(chan struct{})
	agentTool.runnerFactory = func() runner {
		return &blockingRunner{wait: wait}
	}
	defer close(wait)

	call := tool.Call{
		Input: map[string]any{
			"description":       "background agent",
			"prompt":            "analyze tests",
			"subagent_type":     "Explore",
			"run_in_background": true,
		},
	}
	result, err := agentTool.Invoke(context.Background(), call)
	if err != nil {
		t.Fatalf("Invoke() unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("Invoke() error = %q, want empty", result.Error)
	}
	if !strings.Contains(result.Output, "\"status\":\"async_launched\"") {
		t.Fatalf("Invoke() output = %q, want async_launched status", result.Output)
	}

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		tasks := taskStore.list()
		if len(tasks) == 1 {
			if tasks[0].Type != "agent" || tasks[0].Status != coresession.BackgroundTaskStatusRunning {
				t.Fatalf("task snapshot = %+v, want type=agent status=running", tasks[0])
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("expected one running background agent task, got %d", len(taskStore.list()))
}

func TestTool_Invoke_RunInBackgroundFailureKeepsFailedSnapshot(t *testing.T) {
	registry := agent.NewInMemoryRegistry()
	parentRuntime := &engine.Runtime{}
	taskStore := &fakeBackgroundTaskStore{}
	agentTool := NewTool(registry, parentRuntime, nil, nil, taskStore)
	agentTool.runnerFactory = func() runner {
		return &errorRunner{err: errors.New("background failed")}
	}

	call := tool.Call{
		Input: map[string]any{
			"description":       "background failure",
			"prompt":            "analyze tests",
			"subagent_type":     "Explore",
			"run_in_background": true,
		},
	}
	result, err := agentTool.Invoke(context.Background(), call)
	if err != nil {
		t.Fatalf("Invoke() unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("Invoke() error = %q, want empty", result.Error)
	}

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		tasks := taskStore.list()
		if len(tasks) == 1 {
			if tasks[0].Status == coresession.BackgroundTaskStatusFailed {
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	tasks := taskStore.list()
	if len(tasks) != 1 {
		t.Fatalf("expected failed background task snapshot to remain, got %d task(s)", len(tasks))
	}
	t.Fatalf("failed task status = %q, want failed", tasks[0].Status)
}

func TestTool_Invoke_RunInBackgroundCompletionKeepsCompletedSnapshot(t *testing.T) {
	registry := agent.NewInMemoryRegistry()
	parentRuntime := &engine.Runtime{}
	taskStore := &fakeBackgroundTaskStore{}
	agentTool := NewTool(registry, parentRuntime, nil, nil, taskStore)
	agentTool.runnerFactory = func() runner {
		return &successRunner{}
	}

	call := tool.Call{
		Input: map[string]any{
			"description":       "background success",
			"prompt":            "analyze tests",
			"subagent_type":     "Explore",
			"run_in_background": true,
		},
	}
	result, err := agentTool.Invoke(context.Background(), call)
	if err != nil {
		t.Fatalf("Invoke() unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("Invoke() error = %q, want empty", result.Error)
	}

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		tasks := taskStore.list()
		if len(tasks) == 1 {
			if tasks[0].Status == coresession.BackgroundTaskStatusCompleted {
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	tasks := taskStore.list()
	if len(tasks) != 1 {
		t.Fatalf("expected completed background task snapshot to remain, got %d task(s)", len(tasks))
	}
	t.Fatalf("completed task status = %q, want completed", tasks[0].Status)
}

func TestTool_Invoke_RunInBackgroundCanBeStopped(t *testing.T) {
	registry := agent.NewInMemoryRegistry()
	parentRuntime := &engine.Runtime{}
	taskStore := runtimesession.NewBackgroundTaskStore()
	agentTool := NewTool(registry, parentRuntime, nil, nil, taskStore)
	agentTool.runnerFactory = func() runner {
		return &ctxBlockingRunner{}
	}

	call := tool.Call{
		Input: map[string]any{
			"description":       "background stoppable",
			"prompt":            "wait",
			"subagent_type":     "Explore",
			"run_in_background": true,
		},
	}
	result, err := agentTool.Invoke(context.Background(), call)
	if err != nil {
		t.Fatalf("Invoke() unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("Invoke() error = %q, want empty", result.Error)
	}

	var payload map[string]any
	if unmarshalErr := json.Unmarshal([]byte(result.Output), &payload); unmarshalErr != nil {
		t.Fatalf("Unmarshal(output) error = %v", unmarshalErr)
	}
	taskID, _ := payload["agentId"].(string)
	if strings.TrimSpace(taskID) == "" {
		t.Fatalf("agentId missing in output: %s", result.Output)
	}

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if snapshot, ok := taskStore.Get(taskID); ok {
			if snapshot.Status != coresession.BackgroundTaskStatusRunning {
				t.Fatalf("running task status = %q, want running", snapshot.Status)
			}
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	stopped, stopErr := taskStore.Stop(taskID)
	if stopErr != nil {
		t.Fatalf("Stop(%q) error = %v", taskID, stopErr)
	}
	if stopped.Status != coresession.BackgroundTaskStatusStopped {
		t.Fatalf("stopped status = %q, want stopped", stopped.Status)
	}
	snapshot, ok := taskStore.Get(taskID)
	if !ok {
		t.Fatalf("task %q should remain queryable after stop", taskID)
	}
	if snapshot.Status != coresession.BackgroundTaskStatusStopped {
		t.Fatalf("stored task status = %q, want stopped", snapshot.Status)
	}
	if snapshot.ControlsAvailable {
		t.Fatalf("stored task controls_available = true, want false")
	}
}
