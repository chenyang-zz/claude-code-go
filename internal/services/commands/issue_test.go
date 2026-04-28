package commands

import (
	"context"
	"reflect"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestIssueCommandMetadata verifies /issue is exposed as a hidden command.
func TestIssueCommandMetadata(t *testing.T) {
	meta := IssueCommand{}.Metadata()

	if !reflect.DeepEqual(meta, command.Metadata{
		Name:        "issue",
		Description: "Run internal issue workflow",
		Usage:       "/issue",
		Hidden:      true,
	}) {
		t.Fatalf("Metadata() = %#v, want issue metadata", meta)
	}
}

// TestIssueCommandExecute verifies /issue returns the stable fallback for no-arg execution.
func TestIssueCommandExecute(t *testing.T) {
	result, err := IssueCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != issueCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, issueCommandFallback)
	}
}

// TestIssueCommandExecuteRejectsArgs verifies /issue accepts no arguments.
func TestIssueCommandExecuteRejectsArgs(t *testing.T) {
	_, err := IssueCommand{}.Execute(context.Background(), command.Args{RawLine: "now"})
	if err == nil {
		t.Fatal("Execute() error = nil, want usage error")
	}
	if err.Error() != "usage: /issue" {
		t.Fatalf("Execute() error = %q, want usage error", err.Error())
	}
}
