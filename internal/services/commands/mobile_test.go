package commands

import (
	"context"
	"reflect"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestMobileCommandMetadata verifies /mobile is exposed with the expected canonical descriptor.
func TestMobileCommandMetadata(t *testing.T) {
	meta := MobileCommand{}.Metadata()

	if !reflect.DeepEqual(meta, command.Metadata{
		Name:        "mobile",
		Aliases:     []string{"ios", "android"},
		Description: "Show QR code to download the Claude mobile app",
		Usage:       "/mobile",
	}) {
		t.Fatalf("Metadata() = %#v, want mobile metadata", meta)
	}
}

// TestMobileCommandExecute verifies /mobile returns the stable fallback.
func TestMobileCommandExecute(t *testing.T) {
	result, err := MobileCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != mobileCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, mobileCommandFallback)
	}
}
