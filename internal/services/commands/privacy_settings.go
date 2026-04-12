package commands

import (
	"context"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const privacySettingsURL = "https://claude.ai/settings/data-privacy-controls"

const privacySettingsCommandFallback = "Privacy settings UI is not available in Claude Code Go yet. Review and manage your privacy settings at " + privacySettingsURL + ". Grove qualification checks, privacy settings fetch/update, and dialog rendering remain unmigrated."

// PrivacySettingsCommand exposes the minimum text-only /privacy-settings behavior before Grove and settings-page host integrations exist in the Go runtime.
type PrivacySettingsCommand struct{}

// Metadata returns the canonical slash descriptor for /privacy-settings.
func (c PrivacySettingsCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "privacy-settings",
		Description: "View and update your privacy settings",
		Usage:       "/privacy-settings",
	}
}

// Execute reports the stable privacy-settings fallback supported by the current Go host.
func (c PrivacySettingsCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx
	_ = args

	logger.DebugCF("commands", "rendered privacy-settings command fallback output", map[string]any{
		"grove_available":          false,
		"privacy_settings_ui":      false,
		"settings_fetch_available": false,
	})

	return command.Result{
		Output: privacySettingsCommandFallback,
	}, nil
}
