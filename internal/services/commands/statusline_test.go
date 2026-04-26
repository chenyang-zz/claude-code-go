package commands

import (
	"context"
	"reflect"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestStatuslineCommandMetadata verifies /statusline is exposed with the expected canonical descriptor.
func TestStatuslineCommandMetadata(t *testing.T) {
	meta := StatuslineCommand{}.Metadata()

	if !reflect.DeepEqual(meta, command.Metadata{
		Name:        "statusline",
		Description: "Set up Claude Code's status line UI",
		Usage:       "/statusline [prompt]",
	}) {
		t.Fatalf("Metadata() = %#v, want statusline metadata", meta)
	}
}

// TestStatuslineCommandExecute verifies /statusline returns the stable fallback for no-arg execution.
func TestStatuslineCommandExecute(t *testing.T) {
	result, err := StatuslineCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != statuslineCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, statuslineCommandFallback)
	}
}

// TestStatuslineCommandExecuteAcceptsArgs verifies /statusline accepts optional prompt text.
func TestStatuslineCommandExecuteAcceptsArgs(t *testing.T) {
	result, err := StatuslineCommand{}.Execute(context.Background(), command.Args{RawLine: "from my shell ps1"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != statuslineCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, statuslineCommandFallback)
	}
}
