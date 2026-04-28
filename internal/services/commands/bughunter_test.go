package commands

import (
	"context"
	"reflect"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestBughunterCommandMetadata verifies /bughunter is exposed as a hidden command.
func TestBughunterCommandMetadata(t *testing.T) {
	meta := BughunterCommand{}.Metadata()

	if !reflect.DeepEqual(meta, command.Metadata{
		Name:        "bughunter",
		Description: "Run internal bughunter workflow",
		Usage:       "/bughunter",
		Hidden:      true,
	}) {
		t.Fatalf("Metadata() = %#v, want bughunter metadata", meta)
	}
}

// TestBughunterCommandExecute verifies /bughunter returns the stable fallback for no-arg execution.
func TestBughunterCommandExecute(t *testing.T) {
	result, err := BughunterCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != bughunterCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, bughunterCommandFallback)
	}
}

// TestBughunterCommandExecuteRejectsArgs verifies /bughunter accepts no arguments.
func TestBughunterCommandExecuteRejectsArgs(t *testing.T) {
	_, err := BughunterCommand{}.Execute(context.Background(), command.Args{RawLine: "now"})
	if err == nil {
		t.Fatal("Execute() error = nil, want usage error")
	}
	if err.Error() != "usage: /bughunter" {
		t.Fatalf("Execute() error = %q, want usage error", err.Error())
	}
}
