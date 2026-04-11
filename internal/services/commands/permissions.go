package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// PermissionsCommand renders a stable text summary of the currently resolved permission settings.
type PermissionsCommand struct {
	// Config carries the already-resolved runtime configuration snapshot.
	Config coreconfig.Config
}

// Metadata returns the canonical slash descriptor for /permissions.
func (c PermissionsCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "permissions",
		Aliases:     []string{"allowed-tools"},
		Description: "Manage allow & deny tool permission rules",
		Usage:       "/permissions",
	}
}

// Execute formats the current minimal permission configuration into a stable text block.
func (c PermissionsCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx
	_ = args

	effectiveMode := displayValue(c.Config.Permissions.DefaultMode)
	if strings.TrimSpace(c.Config.Permissions.DefaultMode) == "" {
		effectiveMode = displayValue(c.Config.ApprovalMode)
	}

	lines := []string{
		"Permission settings:",
		fmt.Sprintf("- Default mode: %s", effectiveMode),
		fmt.Sprintf("- Disable bypass-permissions mode: %s", permissionsDisableBypassStatus(c.Config.Permissions.DisableBypassPermissionsMode)),
		fmt.Sprintf("- Allow rules: %s", permissionsListSummary(c.Config.Permissions.Allow)),
		fmt.Sprintf("- Deny rules: %s", permissionsListSummary(c.Config.Permissions.Deny)),
		fmt.Sprintf("- Ask rules: %s", permissionsListSummary(c.Config.Permissions.Ask)),
		fmt.Sprintf("- Additional directories: %s", permissionsListSummary(c.Config.Permissions.AdditionalDirectories)),
		"Interactive permission rule editing is not available in the Go host yet. Update .claude/settings.json to change these values.",
	}

	logger.DebugCF("commands", "rendered permissions command output", map[string]any{
		"default_mode":                 effectiveMode,
		"allow_rule_count":             len(c.Config.Permissions.Allow),
		"deny_rule_count":              len(c.Config.Permissions.Deny),
		"ask_rule_count":               len(c.Config.Permissions.Ask),
		"additional_directory_count":   len(c.Config.Permissions.AdditionalDirectories),
		"disable_bypass_permissions":   c.Config.Permissions.DisableBypassPermissionsMode == "disable",
		"approval_mode_config_present": c.Config.ApprovalMode != "",
	})

	return command.Result{
		Output: strings.Join(lines, "\n"),
	}, nil
}

// permissionsDisableBypassStatus normalizes the disable literal into a stable summary string.
func permissionsDisableBypassStatus(value string) string {
	if strings.TrimSpace(value) == "disable" {
		return "enabled"
	}
	return "disabled"
}

// permissionsListSummary renders zero-or-more configured permission entries into one stable line.
func permissionsListSummary(values []string) string {
	if len(values) == 0 {
		return "(none)"
	}
	return strings.Join(values, ", ")
}
