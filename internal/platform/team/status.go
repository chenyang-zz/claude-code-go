package team

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/agent"
	"github.com/sheepzhao/claude-code-go/internal/core/task"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const teamConfigFileName = "config.json"

// StatusProvider loads the current team status summary for /agents.
type StatusProvider interface {
	// CurrentTeamStatus returns the active team summary or nil when no team config exists.
	CurrentTeamStatus(ctx context.Context) (*Status, error)
}

// Reader loads team configuration files and converts them into agent status summaries.
type Reader struct {
	// HomeDir overrides the Claude config home directory when set.
	HomeDir string
	// TaskStore provides the TodoV2 task list used to resolve owner status.
	TaskStore task.Store
}

// TeamFile describes the subset of the team config needed for status summaries.
type TeamFile struct {
	// Name is the human-readable team name from config.json.
	Name string `json:"name"`
	// LeadAgentID identifies the lead agent for the team.
	LeadAgentID string `json:"leadAgentId"`
	// Members contains the team members shown by /agents.
	Members []TeamMember `json:"members"`
}

// TeamMember describes one member entry in the team config.
type TeamMember struct {
	// AgentID is the deterministic identifier used in task ownership.
	AgentID string `json:"agentId"`
	// Name is the human-readable teammate name.
	Name string `json:"name"`
	// AgentType identifies the agent type for display.
	AgentType string `json:"agentType,omitempty"`
}

// Status summarizes one team's current agent states.
type Status struct {
	// TeamName is the readable team name shown to the user.
	TeamName string
	// LeadAgentID identifies the team lead.
	LeadAgentID string
	// Members contains the resolved agent statuses in team order.
	Members []agent.Status
}

// NewReader creates a team status reader for the given home directory and task store.
func NewReader(homeDir string, taskStore task.Store) *Reader {
	return &Reader{
		HomeDir:   homeDir,
		TaskStore: taskStore,
	}
}

// CurrentTeamStatus loads the current team config and resolves agent busy/idle state.
func (r *Reader) CurrentTeamStatus(ctx context.Context) (*Status, error) {
	teamID := task.ResolveTaskListID()
	file, err := r.readTeamFile(teamID)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	tasks, err := r.listTasks(ctx)
	if err != nil {
		return nil, err
	}

	return buildStatus(teamID, file, tasks), nil
}

// readTeamFile loads the current team's config.json from the Claude config home directory.
func (r *Reader) readTeamFile(teamID string) (*TeamFile, error) {
	path := teamConfigPath(r.homeDir(), teamID)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			logger.DebugCF("team.status", "team config missing", map[string]any{
				"path": path,
			})
		}
		return nil, err
	}

	var file TeamFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("team status: parse %s: %w", path, err)
	}
	return &file, nil
}

// listTasks returns the current TodoV2 task list, or nil when the store is unavailable.
func (r *Reader) listTasks(ctx context.Context) ([]*task.Task, error) {
	if r == nil || r.TaskStore == nil {
		return nil, nil
	}
	return r.TaskStore.List(ctx)
}

// homeDir returns the effective Claude config home directory for the reader.
func (r *Reader) homeDir() string {
	if r != nil && strings.TrimSpace(r.HomeDir) != "" {
		return r.HomeDir
	}
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return "."
	}
	return home
}

// teamConfigPath returns the full path to a team's config.json file.
func teamConfigPath(homeDir, teamID string) string {
	return filepath.Join(homeDir, ".claude", "teams", sanitizeName(teamID), teamConfigFileName)
}

// sanitizeName rewrites a string for safe use as a path component.
func sanitizeName(name string) string {
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

// buildStatus constructs a Status from a team file and a task list.
func buildStatus(teamID string, file *TeamFile, tasks []*task.Task) *Status {
	if file == nil {
		return nil
	}

	unresolvedByOwner := make(map[string][]string)
	for _, t := range tasks {
		if t == nil || t.Status == task.StatusCompleted {
			continue
		}
		owner := strings.TrimSpace(t.Owner)
		if owner == "" {
			continue
		}
		unresolvedByOwner[owner] = append(unresolvedByOwner[owner], t.ID)
	}

	members := make([]agent.Status, 0, len(file.Members))
	for _, member := range file.Members {
		currentTasks := uniqueTaskIDs(unresolvedByOwner[member.Name], unresolvedByOwner[member.AgentID])
		sort.Strings(currentTasks)
		status := agent.StatusIdle
		if len(currentTasks) > 0 {
			status = agent.StatusBusy
		}
		members = append(members, agent.Status{
			AgentID:      member.AgentID,
			Name:         member.Name,
			AgentType:    member.AgentType,
			Status:       status,
			CurrentTasks: currentTasks,
		})
	}

	teamName := strings.TrimSpace(file.Name)
	if teamName == "" {
		teamName = teamID
	}
	return &Status{
		TeamName:    teamName,
		LeadAgentID: file.LeadAgentID,
		Members:     members,
	}
}

// uniqueTaskIDs merges and de-duplicates task IDs while preserving the first occurrence order.
func uniqueTaskIDs(groups ...[]string) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0)
	for _, group := range groups {
		for _, id := range group {
			if id == "" {
				continue
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			out = append(out, id)
		}
	}
	return out
}
