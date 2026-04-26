package commands

import (
	"context"
	"reflect"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestRateLimitOptionsCommandMetadata verifies /rate-limit-options is exposed as hidden with the expected descriptor.
func TestRateLimitOptionsCommandMetadata(t *testing.T) {
	meta := RateLimitOptionsCommand{}.Metadata()

	if !reflect.DeepEqual(meta, command.Metadata{
		Name:        "rate-limit-options",
		Description: "Show options when rate limit is reached",
		Usage:       "/rate-limit-options",
		Hidden:      true,
	}) {
		t.Fatalf("Metadata() = %#v, want rate-limit-options metadata", meta)
	}
}

// TestRateLimitOptionsCommandExecute verifies /rate-limit-options returns the stable hidden fallback.
func TestRateLimitOptionsCommandExecute(t *testing.T) {
	result, err := RateLimitOptionsCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != rateLimitOptionsCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, rateLimitOptionsCommandFallback)
	}
}
