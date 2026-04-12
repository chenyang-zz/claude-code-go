package commands

import (
	"context"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestPrivacySettingsCommandMetadata verifies /privacy-settings is exposed with the expected canonical descriptor.
func TestPrivacySettingsCommandMetadata(t *testing.T) {
	meta := PrivacySettingsCommand{}.Metadata()

	if meta.Name != "privacy-settings" {
		t.Fatalf("Metadata().Name = %q, want privacy-settings", meta.Name)
	}
	if meta.Description != "View and update your privacy settings" {
		t.Fatalf("Metadata().Description = %q, want privacy-settings description", meta.Description)
	}
	if meta.Usage != "/privacy-settings" {
		t.Fatalf("Metadata().Usage = %q, want /privacy-settings", meta.Usage)
	}
}

// TestPrivacySettingsCommandExecute verifies /privacy-settings returns the stable fallback guidance.
func TestPrivacySettingsCommandExecute(t *testing.T) {
	result, err := PrivacySettingsCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != privacySettingsCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, privacySettingsCommandFallback)
	}
}
