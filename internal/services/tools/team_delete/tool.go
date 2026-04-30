package team_delete

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	"github.com/sheepzhao/claude-code-go/internal/platform/team"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const Name = "TeamDelete"

// Tool implements the TeamDelete tool for cleaning up team and task directories
// when swarm work is complete.
type Tool struct {
	homeDir string
}

// NewTool creates a TeamDelete tool that removes team data under the given
// home directory.
func NewTool(homeDir string) *Tool {
	return &Tool{homeDir: homeDir}
}

func (t *Tool) Name() string { return Name }

func (t *Tool) Description() string {
	return "Clean up team and task directories when the swarm is complete. " +
		"Provide the team_name to remove its configuration and task directories."
}

// Input defines the TeamDelete tool input schema.
type Input struct {
	TeamName string `json:"team_name"`
}

// Output defines the TeamDelete tool output.
type Output struct {
	Success  bool   `json:"success"`
	Message  string `json:"message"`
	TeamName string `json:"team_name,omitempty"`
}

func (t *Tool) InputSchema() coretool.InputSchema {
	return coretool.InputSchema{
		Properties: map[string]coretool.FieldSchema{
			"team_name": {
				Type:        coretool.ValueKindString,
				Description: "Name of the team to delete.",
				Required:    true,
			},
		},
	}
}

func (t *Tool) IsReadOnly() bool       { return false }
func (t *Tool) IsConcurrencySafe() bool { return false }

// Invoke removes the team and task directories for the given team name.
func (t *Tool) Invoke(ctx context.Context, call coretool.Call) (coretool.Result, error) {
	input, err := coretool.DecodeInput[Input](t.InputSchema(), call.Input)
	if err != nil {
		return coretool.Result{Error: fmt.Sprintf("invalid input: %v", err)}, nil
	}

	teamName := strings.TrimSpace(input.TeamName)
	if teamName == "" {
		return coretool.Result{Error: "team_name is required for TeamDelete"}, nil
	}

	// Verify the team exists before attempting cleanup.
	if existing, _ := team.ReadTeamFile(t.homeDir, teamName); existing == nil {
		data, _ := json.Marshal(Output{
			Success:  true,
			Message:  fmt.Sprintf("team %q does not exist, nothing to clean up", teamName),
			TeamName: teamName,
		})
		return coretool.Result{Output: string(data)}, nil
	}

	team.DeleteTeamDirectories(t.homeDir, teamName)

	logger.InfoCF("team_delete", "team deleted", map[string]any{
		"team_name": teamName,
	})

	data, _ := json.Marshal(Output{
		Success:  true,
		Message:  fmt.Sprintf("Cleaned up directories and worktrees for team %q", teamName),
		TeamName: teamName,
	})
	return coretool.Result{Output: string(data)}, nil
}
