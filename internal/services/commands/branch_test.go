package commands

import (
	"context"
	"reflect"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestBranchCommandMetadata verifies /branch is exposed with the expected canonical descriptor.
func TestBranchCommandMetadata(t *testing.T) {
	meta := BranchCommand{}.Metadata()

	if !reflect.DeepEqual(meta, command.Metadata{
		Name:        "branch",
		Aliases:     []string{"fork"},
		Description: "Create a branch of the current conversation at this point",
		Usage:       "/branch [name]",
	}) {
		t.Fatalf("Metadata() = %#v, want branch metadata", meta)
	}
}

// TestBranchCommandExecute verifies /branch returns the stable branch fallback.
func TestBranchCommandExecute(t *testing.T) {
	result, err := BranchCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != branchCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, branchCommandFallback)
	}
}
