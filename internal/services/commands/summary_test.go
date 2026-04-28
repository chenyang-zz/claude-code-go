package commands

import (
	"context"
	"reflect"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestSummaryCommandMetadata verifies /summary is exposed as a hidden command.
func TestSummaryCommandMetadata(t *testing.T) {
	meta := SummaryCommand{}.Metadata()

	if !reflect.DeepEqual(meta, command.Metadata{
		Name:        "summary",
		Description: "Show internal summary diagnostics",
		Usage:       "/summary",
		Hidden:      true,
	}) {
		t.Fatalf("Metadata() = %#v, want summary metadata", meta)
	}
}

// TestSummaryCommandExecute verifies /summary returns the stable fallback for no-arg execution.
func TestSummaryCommandExecute(t *testing.T) {
	result, err := SummaryCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != summaryCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, summaryCommandFallback)
	}
}

// TestSummaryCommandExecuteRejectsArgs verifies /summary accepts no arguments.
func TestSummaryCommandExecuteRejectsArgs(t *testing.T) {
	_, err := SummaryCommand{}.Execute(context.Background(), command.Args{RawLine: "full"})
	if err == nil {
		t.Fatal("Execute() error = nil, want usage error")
	}
	if err.Error() != "usage: /summary" {
		t.Fatalf("Execute() error = %q, want usage error", err.Error())
	}
}
