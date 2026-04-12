package commands

import (
	"context"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
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

// TestTasksCommandExecute verifies /tasks returns the stable fallback guidance.
func TestTasksCommandExecute(t *testing.T) {
	result, err := TasksCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Output != tasksCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, tasksCommandFallback)
	}
}
