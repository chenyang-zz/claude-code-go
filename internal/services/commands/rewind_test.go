package commands

import (
	"context"
	"reflect"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestRewindCommandMetadata verifies /rewind is exposed with the expected canonical descriptor.
func TestRewindCommandMetadata(t *testing.T) {
	meta := RewindCommand{}.Metadata()

	if !reflect.DeepEqual(meta, command.Metadata{
		Name:        "rewind",
		Aliases:     []string{"checkpoint"},
		Description: "Restore the code and/or conversation to a previous point",
		Usage:       "/rewind",
	}) {
		t.Fatalf("Metadata() = %#v, want rewind metadata", meta)
	}
}

// TestRewindCommandExecute verifies /rewind returns the stable fallback.
func TestRewindCommandExecute(t *testing.T) {
	result, err := RewindCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != rewindCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, rewindCommandFallback)
	}
}
