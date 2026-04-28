package commands

import (
	"context"
	"reflect"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestCtxVizCommandMetadata verifies /ctx_viz is exposed as a hidden command.
func TestCtxVizCommandMetadata(t *testing.T) {
	meta := CtxVizCommand{}.Metadata()

	if !reflect.DeepEqual(meta, command.Metadata{
		Name:        "ctx_viz",
		Description: "Run internal context visualization workflow",
		Usage:       "/ctx_viz",
		Hidden:      true,
	}) {
		t.Fatalf("Metadata() = %#v, want ctx_viz metadata", meta)
	}
}

// TestCtxVizCommandExecute verifies /ctx_viz returns the stable fallback for no-arg execution.
func TestCtxVizCommandExecute(t *testing.T) {
	result, err := CtxVizCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != ctxVizCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, ctxVizCommandFallback)
	}
}

// TestCtxVizCommandExecuteRejectsArgs verifies /ctx_viz accepts no arguments.
func TestCtxVizCommandExecuteRejectsArgs(t *testing.T) {
	_, err := CtxVizCommand{}.Execute(context.Background(), command.Args{RawLine: "now"})
	if err == nil {
		t.Fatal("Execute() error = nil, want usage error")
	}
	if err.Error() != "usage: /ctx_viz" {
		t.Fatalf("Execute() error = %q, want usage error", err.Error())
	}
}
