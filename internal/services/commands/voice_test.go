package commands

import (
	"context"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestVoiceCommandMetadata verifies /voice is exposed with the expected canonical descriptor.
func TestVoiceCommandMetadata(t *testing.T) {
	meta := VoiceCommand{}.Metadata()

	if meta.Name != "voice" {
		t.Fatalf("Metadata().Name = %q, want voice", meta.Name)
	}
	if meta.Description != "Toggle voice mode" {
		t.Fatalf("Metadata().Description = %q, want voice description", meta.Description)
	}
	if meta.Usage != "/voice" {
		t.Fatalf("Metadata().Usage = %q, want /voice", meta.Usage)
	}
}

// TestVoiceCommandExecute verifies /voice returns the stable voice fallback.
func TestVoiceCommandExecute(t *testing.T) {
	result, err := VoiceCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != voiceCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, voiceCommandFallback)
	}
}
