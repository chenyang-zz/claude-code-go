package commands

import (
	"context"
	"reflect"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestDebugToolCallCommandMetadata verifies /debug-tool-call is exposed as a hidden command.
func TestDebugToolCallCommandMetadata(t *testing.T) {
	meta := DebugToolCallCommand{}.Metadata()

	if !reflect.DeepEqual(meta, command.Metadata{
		Name:        "debug-tool-call",
		Description: "Run internal tool-call debug workflow",
		Usage:       "/debug-tool-call",
		Hidden:      true,
	}) {
		t.Fatalf("Metadata() = %#v, want debug-tool-call metadata", meta)
	}
}

// TestDebugToolCallCommandExecute verifies /debug-tool-call returns the stable fallback for no-arg execution.
func TestDebugToolCallCommandExecute(t *testing.T) {
	result, err := DebugToolCallCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != debugToolCallCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, debugToolCallCommandFallback)
	}
}

// TestDebugToolCallCommandExecuteRejectsArgs verifies /debug-tool-call accepts no arguments.
func TestDebugToolCallCommandExecuteRejectsArgs(t *testing.T) {
	_, err := DebugToolCallCommand{}.Execute(context.Background(), command.Args{RawLine: "now"})
	if err == nil {
		t.Fatal("Execute() error = nil, want usage error")
	}
	if err.Error() != "usage: /debug-tool-call" {
		t.Fatalf("Execute() error = %q, want usage error", err.Error())
	}
}
