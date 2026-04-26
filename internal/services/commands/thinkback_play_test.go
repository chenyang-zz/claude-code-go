package commands

import (
	"context"
	"reflect"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestThinkbackPlayCommandMetadata verifies /thinkback-play is exposed as a hidden command.
func TestThinkbackPlayCommandMetadata(t *testing.T) {
	meta := ThinkbackPlayCommand{}.Metadata()

	if !reflect.DeepEqual(meta, command.Metadata{
		Name:        "thinkback-play",
		Description: "Play the thinkback animation",
		Usage:       "/thinkback-play",
		Hidden:      true,
	}) {
		t.Fatalf("Metadata() = %#v, want thinkback-play metadata", meta)
	}
}

// TestThinkbackPlayCommandExecute verifies /thinkback-play returns the stable fallback for no-arg execution.
func TestThinkbackPlayCommandExecute(t *testing.T) {
	result, err := ThinkbackPlayCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != thinkbackPlayCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, thinkbackPlayCommandFallback)
	}
}

// TestThinkbackPlayCommandExecuteRejectsArgs verifies /thinkback-play accepts no arguments.
func TestThinkbackPlayCommandExecuteRejectsArgs(t *testing.T) {
	_, err := ThinkbackPlayCommand{}.Execute(context.Background(), command.Args{RawLine: "play"})
	if err == nil {
		t.Fatal("Execute() error = nil, want usage error")
	}
	if err.Error() != "usage: /thinkback-play" {
		t.Fatalf("Execute() error = %q, want usage error", err.Error())
	}
}
