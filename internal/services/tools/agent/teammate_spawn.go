package agent

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/tool"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// teammateSpawnRequest carries the minimal process-spawn inputs for one teammate launch.
type teammateSpawnRequest struct {
	// Name stores the logical teammate name.
	Name string
	// TeamName stores the logical team identifier.
	TeamName string
	// Cwd overrides the process working directory when non-empty.
	Cwd string
	// SessionConfig carries the parent session snapshot used for CLI flag pass-through.
	SessionConfig tool.SessionConfigSnapshot
}

// teammateStarter abstracts process spawning so tests can validate command assembly.
type teammateStarter interface {
	// Start launches one teammate process and returns its PID when available.
	Start(ctx context.Context, req teammateSpawnRequest) (int, error)
}

// osTeammateStarter spawns teammate processes via os/exec.
type osTeammateStarter struct{}

// Start launches one detached teammate process and returns the created PID.
func (osTeammateStarter) Start(ctx context.Context, req teammateSpawnRequest) (int, error) {
	executablePath, err := resolveCurrentExecutablePath()
	if err != nil {
		return 0, fmt.Errorf("resolve executable failed: %w", err)
	}

	args := teammateSpawnCLIArgs(req.SessionConfig)

	cmd := exec.CommandContext(ctx, executablePath, args...)
	if strings.TrimSpace(req.Cwd) != "" {
		cmd.Dir = req.Cwd
	}
	cmd.Env = append(os.Environ(),
		"CLAUDE_CODE_TEAM_NAME="+req.TeamName,
		"CLAUDE_CODE_AGENT_NAME="+req.Name,
	)
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	pid := 0
	if cmd.Process != nil {
		pid = cmd.Process.Pid
		if releaseErr := cmd.Process.Release(); releaseErr != nil {
			logger.WarnCF("agent.tool", "failed to release teammate process handle", map[string]any{
				"team_name": req.TeamName,
				"name":      req.Name,
				"pid":       pid,
				"error":     releaseErr.Error(),
			})
		}
	}
	return pid, nil
}

// resolveCurrentExecutablePath resolves the current process executable path.
var resolveCurrentExecutablePath = os.Executable

// teammateSpawnCLIArgs builds CLI args propagated from parent session to teammate process.
func teammateSpawnCLIArgs(sessionConfig tool.SessionConfigSnapshot) []string {
	args := make([]string, 0, 2)
	if sessionConfig.HasSettingSourcesFlag {
		args = append(args, "--setting-sources", sessionConfig.SettingSourcesFlag)
	}
	return args
}
