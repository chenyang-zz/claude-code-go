package commands

import (
	"context"
	"reflect"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestPerfIssueCommandMetadata verifies /perf-issue is exposed as a hidden command.
func TestPerfIssueCommandMetadata(t *testing.T) {
	meta := PerfIssueCommand{}.Metadata()

	if !reflect.DeepEqual(meta, command.Metadata{
		Name:        "perf-issue",
		Description: "Run internal performance issue workflow",
		Usage:       "/perf-issue",
		Hidden:      true,
	}) {
		t.Fatalf("Metadata() = %#v, want perf-issue metadata", meta)
	}
}

// TestPerfIssueCommandExecute verifies /perf-issue returns the stable fallback for no-arg execution.
func TestPerfIssueCommandExecute(t *testing.T) {
	result, err := PerfIssueCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != perfIssueCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, perfIssueCommandFallback)
	}
}

// TestPerfIssueCommandExecuteRejectsArgs verifies /perf-issue accepts no arguments.
func TestPerfIssueCommandExecuteRejectsArgs(t *testing.T) {
	_, err := PerfIssueCommand{}.Execute(context.Background(), command.Args{RawLine: "now"})
	if err == nil {
		t.Fatal("Execute() error = nil, want usage error")
	}
	if err.Error() != "usage: /perf-issue" {
		t.Fatalf("Execute() error = %q, want usage error", err.Error())
	}
}
