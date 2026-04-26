package commands

import (
	"context"
	"reflect"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestStickersCommandMetadata verifies /stickers is exposed with the expected canonical descriptor.
func TestStickersCommandMetadata(t *testing.T) {
	meta := StickersCommand{}.Metadata()

	if !reflect.DeepEqual(meta, command.Metadata{
		Name:        "stickers",
		Description: "Order Claude Code stickers",
		Usage:       "/stickers",
	}) {
		t.Fatalf("Metadata() = %#v, want stickers metadata", meta)
	}
}

// TestStickersCommandExecute verifies /stickers returns the stable fallback.
func TestStickersCommandExecute(t *testing.T) {
	result, err := StickersCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != stickersCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, stickersCommandFallback)
	}
}
