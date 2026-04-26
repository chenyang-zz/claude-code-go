package commands

import (
	"context"
	"reflect"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestBtwCommandMetadata verifies /btw is exposed with the expected canonical descriptor.
func TestBtwCommandMetadata(t *testing.T) {
	meta := BtwCommand{}.Metadata()

	if !reflect.DeepEqual(meta, command.Metadata{
		Name:        "btw",
		Description: "Ask a quick side question without interrupting the main conversation",
		Usage:       "/btw <question>",
	}) {
		t.Fatalf("Metadata() = %#v, want btw metadata", meta)
	}
}

// TestBtwCommandExecute verifies /btw accepts one question argument and returns the stable fallback.
func TestBtwCommandExecute(t *testing.T) {
	result, err := BtwCommand{}.Execute(context.Background(), command.Args{RawLine: "what changed in this branch?"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != btwCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, btwCommandFallback)
	}
}

// TestBtwCommandExecuteRequiresQuestion verifies /btw enforces one question argument.
func TestBtwCommandExecuteRequiresQuestion(t *testing.T) {
	_, err := BtwCommand{}.Execute(context.Background(), command.Args{})
	if err == nil {
		t.Fatal("Execute() error = nil, want usage error")
	}
	if err.Error() != "usage: /btw <question>" {
		t.Fatalf("Execute() error = %q, want usage error", err.Error())
	}
}
