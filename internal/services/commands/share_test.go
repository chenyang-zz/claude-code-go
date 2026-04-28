package commands

import (
	"context"
	"reflect"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestShareCommandMetadata verifies /share is exposed as a hidden command.
func TestShareCommandMetadata(t *testing.T) {
	meta := ShareCommand{}.Metadata()

	if !reflect.DeepEqual(meta, command.Metadata{
		Name:        "share",
		Description: "Share the current session",
		Usage:       "/share",
		Hidden:      true,
	}) {
		t.Fatalf("Metadata() = %#v, want share metadata", meta)
	}
}

// TestShareCommandExecute verifies /share returns the stable fallback for no-arg execution.
func TestShareCommandExecute(t *testing.T) {
	result, err := ShareCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != shareCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, shareCommandFallback)
	}
}

// TestShareCommandExecuteRejectsArgs verifies /share accepts no arguments.
func TestShareCommandExecuteRejectsArgs(t *testing.T) {
	_, err := ShareCommand{}.Execute(context.Background(), command.Args{RawLine: "now"})
	if err == nil {
		t.Fatal("Execute() error = nil, want usage error")
	}
	if err.Error() != "usage: /share" {
		t.Fatalf("Execute() error = %q, want usage error", err.Error())
	}
}
