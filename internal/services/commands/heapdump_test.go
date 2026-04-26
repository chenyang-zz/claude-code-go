package commands

import (
	"context"
	"reflect"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestHeapdumpCommandMetadata verifies /heapdump is exposed as a hidden command.
func TestHeapdumpCommandMetadata(t *testing.T) {
	meta := HeapdumpCommand{}.Metadata()

	if !reflect.DeepEqual(meta, command.Metadata{
		Name:        "heapdump",
		Description: "Dump the JS heap to ~/Desktop",
		Usage:       "/heapdump",
		Hidden:      true,
	}) {
		t.Fatalf("Metadata() = %#v, want heapdump metadata", meta)
	}
}

// TestHeapdumpCommandExecute verifies /heapdump returns the stable fallback for no-arg execution.
func TestHeapdumpCommandExecute(t *testing.T) {
	result, err := HeapdumpCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != heapdumpCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, heapdumpCommandFallback)
	}
}

// TestHeapdumpCommandExecuteRejectsArgs verifies /heapdump accepts no arguments.
func TestHeapdumpCommandExecuteRejectsArgs(t *testing.T) {
	_, err := HeapdumpCommand{}.Execute(context.Background(), command.Args{RawLine: "now"})
	if err == nil {
		t.Fatal("Execute() error = nil, want usage error")
	}
	if err.Error() != "usage: /heapdump" {
		t.Fatalf("Execute() error = %q, want usage error", err.Error())
	}
}
