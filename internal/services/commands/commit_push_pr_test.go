package commands

import (
	"context"
	"reflect"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestCommitPushPRCommandMetadata verifies /commit-push-pr is exposed with the expected canonical descriptor.
func TestCommitPushPRCommandMetadata(t *testing.T) {
	meta := CommitPushPRCommand{}.Metadata()

	if !reflect.DeepEqual(meta, command.Metadata{
		Name:        "commit-push-pr",
		Description: "Commit, push, and open a PR",
		Usage:       "/commit-push-pr [instructions]",
	}) {
		t.Fatalf("Metadata() = %#v, want commit-push-pr metadata", meta)
	}
}

// TestCommitPushPRCommandExecute verifies /commit-push-pr returns the stable fallback for no-arg execution.
func TestCommitPushPRCommandExecute(t *testing.T) {
	result, err := CommitPushPRCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != commitPushPRCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, commitPushPRCommandFallback)
	}
}

// TestCommitPushPRCommandExecuteAcceptsArgs verifies /commit-push-pr accepts optional user instructions.
func TestCommitPushPRCommandExecuteAcceptsArgs(t *testing.T) {
	result, err := CommitPushPRCommand{}.Execute(context.Background(), command.Args{RawLine: "skip changelog"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != commitPushPRCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, commitPushPRCommandFallback)
	}
}
