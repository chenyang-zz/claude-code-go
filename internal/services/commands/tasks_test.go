package commands

import (
	"context"
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
	if meta.Usage != "/tasks" {
		t.Fatalf("Metadata().Usage = %q, want /tasks", meta.Usage)
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

	want := "Background tasks: 2\n- task-1: shell - running - build watcher\n- task-2: agent - pending - code review draft\nTask listing is available, but stop/resume controls are not migrated yet."
	if result.Output != want {
		t.Fatalf("Execute() output = %q, want %q", result.Output, want)
	}
}

// TestTasksCommandExecuteWithControllableTasks verifies /tasks reports stop availability for controllable local bash tasks.
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

	want := "Background tasks: 1\n- task-1: bash - running - npm run dev\nControls: stop available for local bash tasks."
	if result.Output != want {
		t.Fatalf("Execute() output = %q, want %q", result.Output, want)
	}
}
