package team

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// ReadTeamFile reads and parses a team's config.json file.
// Returns nil when the file does not exist.
func ReadTeamFile(homeDir, teamName string) (*TeamFile, error) {
	path := TeamFilePath(homeDir, teamName)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("team: read config %s: %w", path, err)
	}

	var file TeamFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("team: parse config %s: %w", path, err)
	}
	return &file, nil
}

// SanitizeName rewrites a string for safe use as a path component.
// Non-alphanumeric characters are replaced with hyphens and the result is lowercased.
func SanitizeName(name string) string {
	var b strings.Builder
	b.Grow(len(name))
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			continue
		}
		b.WriteByte('-')
	}
	return strings.ToLower(b.String())
}

// TeamDir returns the absolute path to a team's directory under ~/.claude/teams/.
func TeamDir(homeDir, teamName string) string {
	return filepath.Join(homeDir, ".claude", "teams", SanitizeName(teamName))
}

// TeamFilePath returns the absolute path to a team's config.json file.
func TeamFilePath(homeDir, teamName string) string {
	return filepath.Join(TeamDir(homeDir, teamName), teamConfigFileName)
}

// TaskDir returns the absolute path to a team's task directory under ~/.claude/tasks/.
func TaskDir(homeDir, teamName string) string {
	return filepath.Join(homeDir, ".claude", "tasks", SanitizeName(teamName))
}

// WriteTeamFile creates the team directory (if needed) and writes the team config.json file.
func WriteTeamFile(homeDir, teamName string, file *TeamFile) error {
	dir := TeamDir(homeDir, teamName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("team: create directory %s: %w", dir, err)
	}

	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return fmt.Errorf("team: marshal config for %s: %w", teamName, err)
	}

	path := TeamFilePath(homeDir, teamName)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("team: write config %s: %w", path, err)
	}

	logger.DebugCF("team", "wrote team config", map[string]any{
		"path": path,
		"name": teamName,
	})
	return nil
}

// DeleteTeamDirectories removes the team config directory and the associated task
// directory for the given team name. Errors during removal are logged but do not
// stop the function from attempting to remove both directories.
func DeleteTeamDirectories(homeDir, teamName string) {
	teamDir := TeamDir(homeDir, teamName)
	if err := os.RemoveAll(teamDir); err != nil {
		logger.DebugCF("team", "failed to remove team directory", map[string]any{
			"path":  teamDir,
			"error": err.Error(),
		})
	}

	taskDir := TaskDir(homeDir, teamName)
	if err := os.RemoveAll(taskDir); err != nil {
		logger.DebugCF("team", "failed to remove task directory", map[string]any{
			"path":  taskDir,
			"error": err.Error(),
		})
	}

	logger.DebugCF("team", "cleaned up team directories", map[string]any{
		"team_name": teamName,
	})
}
