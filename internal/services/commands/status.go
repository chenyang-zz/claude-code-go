package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// StatusCommand renders a minimum host status summary for the current Go CLI runtime.
type StatusCommand struct {
	// Config carries the already-resolved runtime configuration snapshot.
	Config coreconfig.Config
}

// Metadata returns the canonical slash descriptor for /status.
func (c StatusCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "status",
		Description: "Show Claude Code status including version, model, account, API connectivity, and tool statuses",
		Usage:       "/status",
	}
}

// Execute summarizes the stable local status signals that are currently available in the Go host.
func (c StatusCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx
	_ = args

	lines := []string{
		"Status summary:",
		fmt.Sprintf("- Provider: %s", displayValue(c.Config.Provider)),
		fmt.Sprintf("- Model: %s", displayValue(c.Config.Model)),
		fmt.Sprintf("- Project path: %s", displayValue(c.Config.ProjectPath)),
		fmt.Sprintf("- Approval mode: %s", displayValue(c.Config.ApprovalMode)),
		fmt.Sprintf("- Session storage: %s", statusSessionStorage(c.Config.SessionDBPath)),
		fmt.Sprintf("- Account auth: %s", statusAccountAuth(c.Config.APIKey)),
		fmt.Sprintf("- API base URL: %s", baseURLValue(c.Config.APIBaseURL)),
		"- API connectivity check: not available in Claude Code Go yet",
		"- Tool status checks: not available in Claude Code Go yet",
		"- Settings status UI: not available in Claude Code Go yet",
	}

	logger.DebugCF("commands", "rendered status command output", map[string]any{
		"provider":              c.Config.Provider,
		"model":                 c.Config.Model,
		"project_path":          c.Config.ProjectPath,
		"approval_mode":         c.Config.ApprovalMode,
		"has_api_key":           c.Config.APIKey != "",
		"has_session_db_path":   c.Config.SessionDBPath != "",
		"api_connectivity_live": false,
		"tool_status_live":      false,
	})

	return command.Result{
		Output: strings.Join(lines, "\n"),
	}, nil
}

// statusSessionStorage reports whether session persistence is configured without probing the filesystem.
func statusSessionStorage(path string) string {
	if strings.TrimSpace(path) == "" {
		return "not configured"
	}
	return fmt.Sprintf("configured (%s)", path)
}

// statusAccountAuth reports the stable authentication state currently visible to the Go host.
func statusAccountAuth(apiKey string) string {
	if strings.TrimSpace(apiKey) == "" {
		return "missing API key; interactive account status is not available"
	}
	return "API key configured; interactive account status is not available"
}
