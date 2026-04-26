package commands

import (
	"context"
	"reflect"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestChromeCommandMetadata verifies /chrome is exposed with the expected canonical descriptor.
func TestChromeCommandMetadata(t *testing.T) {
	meta := ChromeCommand{}.Metadata()

	if !reflect.DeepEqual(meta, command.Metadata{
		Name:        "chrome",
		Description: "Claude in Chrome (Beta) settings",
		Usage:       "/chrome",
	}) {
		t.Fatalf("Metadata() = %#v, want chrome metadata", meta)
	}
}

// TestChromeCommandExecute verifies /chrome returns the stable fallback for no-arg execution.
func TestChromeCommandExecute(t *testing.T) {
	result, err := ChromeCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != chromeCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, chromeCommandFallback)
	}
}

// TestChromeCommandExecuteRejectsArgs verifies /chrome accepts no arguments.
func TestChromeCommandExecuteRejectsArgs(t *testing.T) {
	_, err := ChromeCommand{}.Execute(context.Background(), command.Args{RawLine: "open"})
	if err == nil {
		t.Fatal("Execute() error = nil, want usage error")
	}
	if err.Error() != "usage: /chrome" {
		t.Fatalf("Execute() error = %q, want usage error", err.Error())
	}
}
