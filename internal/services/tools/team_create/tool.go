package team_create

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	"github.com/sheepzhao/claude-code-go/internal/platform/team"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const Name = "TeamCreate"

// Tool implements the TeamCreate tool for creating a new multi-agent team.
// It creates the team directory, writes config.json, and initializes the
// corresponding task directory.
type Tool struct {
	homeDir string
}

// NewTool creates a TeamCreate tool that writes team configuration under
// the given home directory.
func NewTool(homeDir string) *Tool {
	return &Tool{homeDir: homeDir}
}

func (t *Tool) Name() string { return Name }

func (t *Tool) Description() string {
	return "Create a new team for coordinating multiple agents. " +
		"Each team has its own task list and message inbox. " +
		"Provide a team_name, optional description, and optional agent_type for the team lead."
}

// Input defines the TeamCreate tool input schema.
type Input struct {
	TeamName    string `json:"team_name"`
	Description string `json:"description,omitempty"`
	AgentType   string `json:"agent_type,omitempty"`
}

// Output defines the TeamCreate tool output.
type Output struct {
	TeamName     string `json:"team_name"`
	TeamFilePath string `json:"team_file_path"`
	LeadAgentID  string `json:"lead_agent_id"`
}

func (t *Tool) InputSchema() coretool.InputSchema {
	return coretool.InputSchema{
		Properties: map[string]coretool.FieldSchema{
			"team_name": {
				Type:        coretool.ValueKindString,
				Description: "Name for the new team to create.",
				Required:    true,
			},
			"description": {
				Type:        coretool.ValueKindString,
				Description: "Team description/purpose.",
			},
			"agent_type": {
				Type:        coretool.ValueKindString,
				Description: "Type/role of the team lead (e.g., \"researcher\", \"test-runner\"). Used for team file and inter-agent coordination.",
			},
		},
	}
}

func (t *Tool) IsReadOnly() bool       { return false }
func (t *Tool) IsConcurrencySafe() bool { return false }

// Invoke creates a new team with the given configuration.
func (t *Tool) Invoke(ctx context.Context, call coretool.Call) (coretool.Result, error) {
	input, err := coretool.DecodeInput[Input](t.InputSchema(), call.Input)
	if err != nil {
		return coretool.Result{Error: fmt.Sprintf("invalid input: %v", err)}, nil
	}

	teamName := strings.TrimSpace(input.TeamName)
	if teamName == "" {
		return coretool.Result{Error: "team_name is required for TeamCreate"}, nil
	}

	// Check if team already exists
	_ = team.TeamFilePath(t.homeDir, teamName)
	if existing, _ := team.ReadTeamFile(t.homeDir, teamName); existing != nil {
		return coretool.Result{Error: fmt.Sprintf("team %q already exists at %s", teamName, team.TeamFilePath(t.homeDir, teamName))}, nil
	}

	leadAgentType := strings.TrimSpace(input.AgentType)
	if leadAgentType == "" {
		leadAgentType = "team-lead"
	}

	leadAgentID := fmt.Sprintf("team-lead@%s", teamName)

	teamFile := &team.TeamFile{
		Name:        teamName,
		Description: strings.TrimSpace(input.Description),
		CreatedAt:   time.Now().UnixMilli(),
		LeadAgentID: leadAgentID,
		Members: []team.TeamMember{
			{
				AgentID:   leadAgentID,
				Name:      "team-lead",
				AgentType: leadAgentType,
			},
		},
	}

	if err := team.WriteTeamFile(t.homeDir, teamName, teamFile); err != nil {
		logger.ErrorCF("team_create", "failed to write team file", map[string]any{
			"team_name": teamName,
			"error":     err.Error(),
		})
		return coretool.Result{Error: fmt.Sprintf("failed to create team: %v", err)}, nil
	}

	logger.InfoCF("team_create", "team created", map[string]any{
		"team_name":      teamName,
		"lead_agent_id":  leadAgentID,
		"team_file_path": team.TeamFilePath(t.homeDir, teamName),
	})

	data, _ := json.Marshal(Output{
		TeamName:     teamName,
		TeamFilePath: team.TeamFilePath(t.homeDir, teamName),
		LeadAgentID:  leadAgentID,
	})
	return coretool.Result{Output: string(data)}, nil
}
