package commands

import (
	"context"
	"reflect"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestAntTraceCommandMetadata verifies /ant-trace is exposed as a hidden command.
func TestAntTraceCommandMetadata(t *testing.T) {
	meta := AntTraceCommand{}.Metadata()

	if !reflect.DeepEqual(meta, command.Metadata{
		Name:        "ant-trace",
		Description: "Run internal ANT trace workflow",
		Usage:       "/ant-trace",
		Hidden:      true,
	}) {
		t.Fatalf("Metadata() = %#v, want ant-trace metadata", meta)
	}
}

// TestAntTraceCommandExecute verifies /ant-trace returns the stable fallback for no-arg execution.
func TestAntTraceCommandExecute(t *testing.T) {
	result, err := AntTraceCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != antTraceCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, antTraceCommandFallback)
	}
}

// TestAntTraceCommandExecuteRejectsArgs verifies /ant-trace accepts no arguments.
func TestAntTraceCommandExecuteRejectsArgs(t *testing.T) {
	_, err := AntTraceCommand{}.Execute(context.Background(), command.Args{RawLine: "now"})
	if err == nil {
		t.Fatal("Execute() error = nil, want usage error")
	}
	if err.Error() != "usage: /ant-trace" {
		t.Fatalf("Execute() error = %q, want usage error", err.Error())
	}
}
