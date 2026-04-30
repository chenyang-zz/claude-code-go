package commands

import (
	"context"
	"reflect"
	"strings"
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

// TestReloadPluginsCommandExecute_NoLoader verifies /reload-plugins returns an error when the loader is nil.
func TestReloadPluginsCommandExecute_NoLoader(t *testing.T) {
	_, err := ReloadPluginsCommand{}.Execute(context.Background(), command.Args{})
	if err == nil {
		t.Fatal("Execute() error = nil, want loader unavailable error")
	}
	if !strings.Contains(err.Error(), "plugin loader is not available") {
		t.Fatalf("Execute() error = %q, want loader unavailable error", err.Error())
	}
}

// TestReloadPluginsCommandExecuteRejectsArgs verifies /reload-plugins accepts no arguments.
func TestReloadPluginsCommandExecuteRejectsArgs(t *testing.T) {
	_, err := ReloadPluginsCommand{}.Execute(context.Background(), command.Args{RawLine: "now"})
	if err == nil {
		t.Fatal("Execute() error = nil, want usage error")
	}
	if !strings.Contains(err.Error(), "usage: /reload-plugins") {
		t.Fatalf("Execute() error = %q, want usage error", err.Error())
	}
}
