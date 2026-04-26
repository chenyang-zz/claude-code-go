package commands

import (
	"context"
	"reflect"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestReloadPluginsCommandMetadata verifies /reload-plugins is exposed with the expected canonical descriptor.
func TestReloadPluginsCommandMetadata(t *testing.T) {
	meta := ReloadPluginsCommand{}.Metadata()

	if !reflect.DeepEqual(meta, command.Metadata{
		Name:        "reload-plugins",
		Description: "Activate pending plugin changes in the current session",
		Usage:       "/reload-plugins",
	}) {
		t.Fatalf("Metadata() = %#v, want reload-plugins metadata", meta)
	}
}

// TestReloadPluginsCommandExecute verifies /reload-plugins returns the stable fallback for no-arg execution.
func TestReloadPluginsCommandExecute(t *testing.T) {
	result, err := ReloadPluginsCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != reloadPluginsCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, reloadPluginsCommandFallback)
	}
}

// TestReloadPluginsCommandExecuteRejectsArgs verifies /reload-plugins accepts no arguments.
func TestReloadPluginsCommandExecuteRejectsArgs(t *testing.T) {
	_, err := ReloadPluginsCommand{}.Execute(context.Background(), command.Args{RawLine: "now"})
	if err == nil {
		t.Fatal("Execute() error = nil, want usage error")
	}
	if err.Error() != "usage: /reload-plugins" {
		t.Fatalf("Execute() error = %q, want usage error", err.Error())
	}
}
