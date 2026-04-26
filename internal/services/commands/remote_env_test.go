package commands

import (
	"context"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestRemoteEnvCommandMetadata verifies /remote-env is exposed with the expected canonical descriptor.
func TestRemoteEnvCommandMetadata(t *testing.T) {
	meta := RemoteEnvCommand{}.Metadata()

	if meta.Name != "remote-env" {
		t.Fatalf("Metadata().Name = %q, want remote-env", meta.Name)
	}
	if meta.Description != "Configure the default remote environment for teleport sessions" {
		t.Fatalf("Metadata().Description = %q, want remote-env description", meta.Description)
	}
	if meta.Usage != "/remote-env" {
		t.Fatalf("Metadata().Usage = %q, want /remote-env", meta.Usage)
	}
}

// TestRemoteEnvCommandExecute verifies /remote-env returns the stable fallback.
func TestRemoteEnvCommandExecute(t *testing.T) {
	result, err := RemoteEnvCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != remoteEnvCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, remoteEnvCommandFallback)
	}
}
