package commands

import (
	"context"
	"reflect"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestExitCommandMetadata verifies /exit is exposed with the expected canonical descriptor.
func TestExitCommandMetadata(t *testing.T) {
	meta := ExitCommand{}.Metadata()

	if !reflect.DeepEqual(meta, command.Metadata{
		Name:        "exit",
		Aliases:     []string{"quit"},
		Description: "Exit the REPL",
		Usage:       "/exit",
	}) {
		t.Fatalf("Metadata() = %#v, want exit metadata", meta)
	}
}

// TestExitCommandExecute verifies /exit returns the stable fallback.
func TestExitCommandExecute(t *testing.T) {
	result, err := ExitCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != exitCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, exitCommandFallback)
	}
}
