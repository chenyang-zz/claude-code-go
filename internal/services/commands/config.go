package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// ConfigCommand renders a stable text summary of the currently resolved runtime configuration.
type ConfigCommand struct {
	// Config carries the already-resolved runtime configuration snapshot.
	Config coreconfig.Config
}

// Metadata returns the canonical slash descriptor for /config.
func (c ConfigCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "config",
		Aliases:     []string{"settings"},
		Description: "Show the current runtime configuration",
		Usage:       "/config",
	}
}

// Execute formats the current runtime configuration into a stable text block.
func (c ConfigCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx
	_ = args

	lines := []string{
		"Current configuration:",
		fmt.Sprintf("- Provider: %s", displayValue(c.Config.Provider)),
		fmt.Sprintf("- Model: %s", displayValue(c.Config.Model)),
		fmt.Sprintf("- Theme: %s", displayValue(coreconfig.NormalizeThemeSetting(c.Config.Theme))),
		fmt.Sprintf("- Editor mode: %s", displayValue(coreconfig.NormalizeEditorMode(c.Config.EditorMode))),
		fmt.Sprintf("- Project path: %s", displayValue(c.Config.ProjectPath)),
		fmt.Sprintf("- Approval mode: %s", displayValue(c.Config.ApprovalMode)),
		fmt.Sprintf("- Session DB path: %s", displayValue(c.Config.SessionDBPath)),
		fmt.Sprintf("- API key: %s", secretStatus(c.Config.APIKey)),
		fmt.Sprintf("- API base URL: %s", baseURLValue(c.Config.APIBaseURL)),
	}

	logger.DebugCF("commands", "rendered config command output", map[string]any{
		"provider":            c.Config.Provider,
		"model":               c.Config.Model,
		"theme":               coreconfig.NormalizeThemeSetting(c.Config.Theme),
		"editor_mode":         coreconfig.NormalizeEditorMode(c.Config.EditorMode),
		"project_path":        c.Config.ProjectPath,
		"approval_mode":       c.Config.ApprovalMode,
		"has_api_key":         c.Config.APIKey != "",
		"has_session_db_path": c.Config.SessionDBPath != "",
	})

	return command.Result{
		Output: strings.Join(lines, "\n"),
	}, nil
}

// displayValue normalizes empty configuration values into one stable placeholder.
func displayValue(value string) string {
	if strings.TrimSpace(value) == "" {
		return "(not set)"
	}
	return value
}

// secretStatus reports whether a sensitive value is configured without exposing the secret itself.
func secretStatus(value string) string {
	if strings.TrimSpace(value) == "" {
		return "missing"
	}
	return "configured"
}

// baseURLValue reports the custom API base URL or the stable default marker when unset.
func baseURLValue(value string) string {
	if strings.TrimSpace(value) == "" {
		return "default"
	}
	return value
}
