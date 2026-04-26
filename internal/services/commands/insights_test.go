package commands

import (
	"context"
	"reflect"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestInsightsCommandMetadata verifies /insights is exposed with the expected canonical descriptor.
func TestInsightsCommandMetadata(t *testing.T) {
	meta := InsightsCommand{}.Metadata()

	if !reflect.DeepEqual(meta, command.Metadata{
		Name:        "insights",
		Description: "Generate a report analyzing your Claude Code sessions",
		Usage:       "/insights [--homespaces]",
	}) {
		t.Fatalf("Metadata() = %#v, want insights metadata", meta)
	}
}

// TestInsightsCommandExecute verifies /insights returns the stable fallback for no-arg execution.
func TestInsightsCommandExecute(t *testing.T) {
	result, err := InsightsCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != insightsCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, insightsCommandFallback)
	}
}

// TestInsightsCommandExecuteAcceptsHomespacesFlag verifies /insights accepts --homespaces.
func TestInsightsCommandExecuteAcceptsHomespacesFlag(t *testing.T) {
	result, err := InsightsCommand{}.Execute(context.Background(), command.Args{RawLine: "--homespaces"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != insightsCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, insightsCommandFallback)
	}
}

// TestInsightsCommandExecuteRejectsUnsupportedArgs verifies /insights rejects unsupported arguments.
func TestInsightsCommandExecuteRejectsUnsupportedArgs(t *testing.T) {
	_, err := InsightsCommand{}.Execute(context.Background(), command.Args{RawLine: "--unknown"})
	if err == nil {
		t.Fatal("Execute() error = nil, want usage error")
	}
	if err.Error() != "usage: /insights [--homespaces]" {
		t.Fatalf("Execute() error = %q, want usage error", err.Error())
	}
}
