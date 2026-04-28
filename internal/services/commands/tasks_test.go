package commands

import (
	"context"
	"errors"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	coresession "github.com/sheepzhao/claude-code-go/internal/core/session"
	runtimesession "github.com/sheepzhao/claude-code-go/internal/runtime/session"
)

// TestTasksCommandMetadata verifies /tasks is exposed with the expected canonical descriptor.
func TestTasksCommandMetadata(t *testing.T) {
	meta := TasksCommand{}.Metadata()
	if meta.Name != "tasks" {
		t.Fatalf("Metadata().Name = %q, want tasks", meta.Name)
	}
	if len(meta.Aliases) != 1 || meta.Aliases[0] != "bashes" {
		t.Fatalf("Metadata().Aliases = %#v, want [bashes]", meta.Aliases)
	}
	if meta.Description != "List and manage background tasks" {
		t.Fatalf("Metadata().Description = %q, want tasks description", meta.Description)
	}
	if meta.Usage != "/tasks | /tasks stop <task-id> | /tasks resume <task-id> [message]" {
		t.Fatalf("Metadata().Usage = %q, want /tasks | /tasks stop <task-id> | /tasks resume <task-id> [message]", meta.Usage)
	}
}

// TestTasksCommandExecute verifies /tasks reports the stable empty-state guidance when no task snapshots exist.
func TestTasksCommandExecute(t *testing.T) {
	result, err := TasksCommand{TaskStore: runtimesession.NewBackgroundTaskStore()}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Output != tasksCommandEmptyState {
		t.Fatalf("Execute() output = %q, want %q", result.Output, tasksCommandEmptyState)
	}
}

// TestTasksCommandExecuteWithTasks verifies /tasks renders the minimum task list summary from the shared runtime store.
func TestTasksCommandExecuteWithTasks(t *testing.T) {
	store := runtimesession.NewBackgroundTaskStore()
	store.Replace([]coresession.BackgroundTaskSnapshot{
		{
			ID:                "task-1",
			Type:              "shell",
			Status:            coresession.BackgroundTaskStatusRunning,
			Summary:           "build watcher",
			ControlsAvailable: false,
		},
		{
			ID:                "task-2",
			Type:              "agent",
			Status:            coresession.BackgroundTaskStatusPending,
			Summary:           "code review draft",
			ControlsAvailable: false,
		},
	})

	result, err := TasksCommand{TaskStore: store}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	want := "Background tasks: 2\n- task-1: shell - running - build watcher\n- task-2: agent - pending - code review draft\nControls: no stoppable background tasks right now."
	if result.Output != want {
		t.Fatalf("Execute() output = %q, want %q", result.Output, want)
	}
}

// TestTasksCommandExecuteWithControllableTasks verifies /tasks reports stop availability for controllable tasks.
func TestTasksCommandExecuteWithControllableTasks(t *testing.T) {
	store := runtimesession.NewBackgroundTaskStore()
	store.Replace([]coresession.BackgroundTaskSnapshot{
		{
			ID:                "task-1",
			Type:              "bash",
			Status:            coresession.BackgroundTaskStatusRunning,
			Summary:           "npm run dev",
			ControlsAvailable: true,
		},
	})

	result, err := TasksCommand{TaskStore: store}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	want := "Background tasks: 1\n- task-1: bash - running - npm run dev\nControls: stop available via /tasks stop <task-id>."
	if result.Output != want {
		t.Fatalf("Execute() output = %q, want %q", result.Output, want)
	}
}

// TestTasksCommandExecuteWithMixedTaskStatuses verifies stop control stays available when at least one running task is stoppable.
func TestTasksCommandExecuteWithMixedTaskStatuses(t *testing.T) {
	store := runtimesession.NewBackgroundTaskStore()
	store.Replace([]coresession.BackgroundTaskSnapshot{
		{
			ID:                "task-1",
			Type:              "bash",
			Status:            coresession.BackgroundTaskStatusStopped,
			Summary:           "npm run dev",
			ControlsAvailable: false,
		},
		{
			ID:                "task-2",
			Type:              "agent",
			Status:            coresession.BackgroundTaskStatusRunning,
			Summary:           "review draft",
			ControlsAvailable: true,
		},
	})

	result, err := TasksCommand{TaskStore: store}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	want := "Background tasks: 2\n- task-1: bash - stopped - npm run dev\n- task-2: agent - running - review draft\nControls: stop available via /tasks stop <task-id>."
	if result.Output != want {
		t.Fatalf("Execute() output = %q, want %q", result.Output, want)
	}
}

func TestTasksCommandExecuteStopSuccess(t *testing.T) {
	store := runtimesession.NewBackgroundTaskStore()
	store.Register(coresession.BackgroundTaskSnapshot{
		ID:                "task-1",
		Type:              "agent",
		Status:            coresession.BackgroundTaskStatusRunning,
		Summary:           "review draft",
		ControlsAvailable: true,
	}, &recordingStopper{})

	result, err := TasksCommand{TaskStore: store}.Execute(context.Background(), command.Args{
		Raw: []string{"stop", "task-1"},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Output != "Stopped background task: task-1 (review draft)" {
		t.Fatalf("Execute() output = %q", result.Output)
	}
}

func TestTasksCommandExecuteStopUsage(t *testing.T) {
	result, err := TasksCommand{TaskStore: runtimesession.NewBackgroundTaskStore()}.Execute(context.Background(), command.Args{
		Raw: []string{"stop"},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Output != "Usage: /tasks stop <task-id>" {
		t.Fatalf("Execute() output = %q", result.Output)
	}
}

func TestTasksCommandExecuteStopFailure(t *testing.T) {
	store := &stubStopFailStore{err: errors.New("boom")}
	result, err := TasksCommand{TaskStore: store}.Execute(context.Background(), command.Args{
		Raw: []string{"stop", "task-9"},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	want := "Failed to stop task task-9: boom"
	if result.Output != want {
		t.Fatalf("Execute() output = %q, want %q", result.Output, want)
	}
}

func TestTasksCommandExecuteResumeUnavailable(t *testing.T) {
	result, err := TasksCommand{TaskStore: &stubNoResumeStore{}}.Execute(context.Background(), command.Args{
		Raw: []string{"resume", "task-1"},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Output != "Background task resume is unavailable." {
		t.Fatalf("Execute() output = %q", result.Output)
	}
}

func TestTasksCommandExecuteResumeUsage(t *testing.T) {
	store := &stubResumeStore{}
	result, err := TasksCommand{TaskStore: store}.Execute(context.Background(), command.Args{
		Raw: []string{"resume"},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Output != "Usage: /tasks resume <task-id> [message]" {
		t.Fatalf("Execute() output = %q", result.Output)
	}
}

func TestTasksCommandExecuteResumeSuccess(t *testing.T) {
	store := &stubResumeStore{
		snapshot: coresession.BackgroundTaskSnapshot{
			ID:      "task-7",
			Type:    "agent",
			Status:  coresession.BackgroundTaskStatusRunning,
			Summary: "review draft",
		},
	}
	result, err := TasksCommand{TaskStore: store}.Execute(context.Background(), command.Args{
		Raw: []string{"resume", "task-7", "please", "continue"},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Output != "Resumed background task: task-7 (review draft)" {
		t.Fatalf("Execute() output = %q", result.Output)
	}
	if store.resumedTaskID != "task-7" {
		t.Fatalf("resumed task id = %q, want task-7", store.resumedTaskID)
	}
	if store.resumedMessage != "please continue" {
		t.Fatalf("resumed message = %q, want %q", store.resumedMessage, "please continue")
	}
}

func TestTasksCommandExecuteResumeFailure(t *testing.T) {
	store := &stubResumeStore{err: errors.New("boom")}
	result, err := TasksCommand{TaskStore: store}.Execute(context.Background(), command.Args{
		Raw: []string{"resume", "task-2"},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Output != "Failed to resume task task-2: boom" {
		t.Fatalf("Execute() output = %q", result.Output)
	}
}

func TestTasksCommandExecuteResumeWithRuntimeStoreSuccess(t *testing.T) {
	store := runtimesession.NewBackgroundTaskStore()
	stopper := &recordingResumeStopper{}
	store.Register(coresession.BackgroundTaskSnapshot{
		ID:                "task-1",
		Type:              "agent",
		Status:            coresession.BackgroundTaskStatusStopped,
		Summary:           "review draft",
		ControlsAvailable: false,
	}, stopper)

	result, err := TasksCommand{TaskStore: store}.Execute(context.Background(), command.Args{
		Raw: []string{"resume", "task-1", "continue", "please"},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Output != "Resumed background task: task-1 (review draft)" {
		t.Fatalf("Execute() output = %q", result.Output)
	}
	if stopper.resumedMessage != "continue please" {
		t.Fatalf("resume message = %q, want %q", stopper.resumedMessage, "continue please")
	}
}

func TestTasksCommandExecuteResumeWithRuntimeStoreRejectsNonAgent(t *testing.T) {
	store := runtimesession.NewBackgroundTaskStore()
	store.Register(coresession.BackgroundTaskSnapshot{
		ID:                "task-1",
		Type:              "bash",
		Status:            coresession.BackgroundTaskStatusStopped,
		Summary:           "npm run dev",
		ControlsAvailable: false,
	}, &recordingResumeStopper{})

	result, err := TasksCommand{TaskStore: store}.Execute(context.Background(), command.Args{
		Raw: []string{"resume", "task-1"},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Output != "Failed to resume task task-1: background task task-1 does not support resume" {
		t.Fatalf("Execute() output = %q", result.Output)
	}
}

func TestTasksCommandExecuteResumeWithRuntimeStoreRejectsNonStopped(t *testing.T) {
	store := runtimesession.NewBackgroundTaskStore()
	store.Register(coresession.BackgroundTaskSnapshot{
		ID:                "task-1",
		Type:              "agent",
		Status:            coresession.BackgroundTaskStatusRunning,
		Summary:           "review draft",
		ControlsAvailable: true,
	}, &recordingResumeStopper{})

	result, err := TasksCommand{TaskStore: store}.Execute(context.Background(), command.Args{
		Raw: []string{"resume", "task-1"},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Output != "Failed to resume task task-1: background task task-1 is not stopped" {
		t.Fatalf("Execute() output = %q", result.Output)
	}
}

type recordingStopper struct{}

func (s *recordingStopper) Stop() error { return nil }

type stubStopFailStore struct{ err error }

func (s *stubStopFailStore) List() []coresession.BackgroundTaskSnapshot {
	return nil
}

func (s *stubStopFailStore) Stop(id string) (coresession.BackgroundTaskSnapshot, error) {
	_ = id
	return coresession.BackgroundTaskSnapshot{}, s.err
}

type stubResumeStore struct {
	snapshot      coresession.BackgroundTaskSnapshot
	err           error
	resumedTaskID string
	resumedMessage string
}

type stubNoResumeStore struct{}

func (s *stubNoResumeStore) List() []coresession.BackgroundTaskSnapshot {
	return nil
}

func (s *stubNoResumeStore) Stop(id string) (coresession.BackgroundTaskSnapshot, error) {
	_ = id
	return coresession.BackgroundTaskSnapshot{}, errors.New("not implemented")
}

type recordingResumeStopper struct {
	resumedMessage string
}

func (s *recordingResumeStopper) Stop() error { return nil }

func (s *recordingResumeStopper) Resume(message string) error {
	s.resumedMessage = message
	return nil
}

func (s *stubResumeStore) List() []coresession.BackgroundTaskSnapshot {
	return nil
}

func (s *stubResumeStore) Stop(id string) (coresession.BackgroundTaskSnapshot, error) {
	_ = id
	return coresession.BackgroundTaskSnapshot{}, errors.New("not implemented")
}

func (s *stubResumeStore) Resume(id string, message string) (coresession.BackgroundTaskSnapshot, error) {
	s.resumedTaskID = id
	s.resumedMessage = message
	if s.err != nil {
		return coresession.BackgroundTaskSnapshot{}, s.err
	}
	return s.snapshot, nil
}
