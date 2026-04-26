package commands

import (
	"context"
	"reflect"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestCommitCommandMetadata verifies /commit is exposed with the expected canonical descriptor.
func TestCommitCommandMetadata(t *testing.T) {
	meta := CommitCommand{}.Metadata()

	if !reflect.DeepEqual(meta, command.Metadata{
		Name:        "commit",
		Description: "Create a git commit",
		Usage:       "/commit",
	}) {
		t.Fatalf("Metadata() = %#v, want commit metadata", meta)
	}
}

// TestCommitCommandExecute verifies /commit returns the stable fallback for no-arg execution.
func TestCommitCommandExecute(t *testing.T) {
	result, err := CommitCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != commitCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, commitCommandFallback)
	}
}

// TestCommitCommandExecuteAcceptsArgs verifies /commit accepts optional free-form argument text.
func TestCommitCommandExecuteAcceptsArgs(t *testing.T) {
	result, err := CommitCommand{}.Execute(context.Background(), command.Args{RawLine: "focus docs first"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != commitCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, commitCommandFallback)
	}
}
