package commands

import (
	"context"
	"reflect"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestPluginCommandMetadata verifies /plugin is exposed with the expected canonical descriptor and aliases.
func TestPluginCommandMetadata(t *testing.T) {
	meta := PluginCommand{}.Metadata()

	if !reflect.DeepEqual(meta, command.Metadata{
		Name:        "plugin",
		Aliases:     []string{"plugins", "marketplace"},
		Description: "Manage Claude Code plugins",
		Usage:       "/plugin [subcommand]",
	}) {
		t.Fatalf("Metadata() = %#v, want plugin metadata", meta)
	}
}

// TestPluginCommandExecute verifies /plugin returns the stable settings fallback.
func TestPluginCommandExecute(t *testing.T) {
	result, err := PluginCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != pluginCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, pluginCommandFallback)
	}
}
