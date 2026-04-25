package team

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/agent"
	"github.com/sheepzhao/claude-code-go/internal/core/task"
)

// writeTeamConfig creates a minimal team config file for tests.
func writeTeamConfig(t *testing.T, homeDir, teamID string, content string) string {
	t.Helper()
	path := teamConfigPath(homeDir, teamID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return path
}

// TestCurrentTeamStatus verifies the reader resolves member busy/idle state from task ownership.
func TestCurrentTeamStatus(t *testing.T) {
	t.Setenv("CLAUDE_CODE_TASK_LIST_ID", "team-alpha")

	homeDir := t.TempDir()
	taskStore := task.NewFileStore(filepath.Join(homeDir, ".claude", "tasks", "team-alpha"))
	ctx := context.Background()

	if _, err := taskStore.Create(ctx, task.NewTask{
		Subject:     "Implement feature",
		Description: "Work item",
		Owner:       "researcher",
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := taskStore.Create(ctx, task.NewTask{
		Subject:     "Write tests",
		Description: "Validation",
		Owner:       "agent-2",
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	completedID, err := taskStore.Create(ctx, task.NewTask{
		Subject:     "Completed work",
		Description: "Done",
		Owner:       "researcher",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	completed := task.StatusCompleted
	if _, err := taskStore.Update(ctx, completedID, task.Updates{Status: &completed}); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	writeTeamConfig(t, homeDir, "team-alpha", `{
  "name": "alpha-team",
  "leadAgentId": "lead-1",
  "members": [
    {"agentId": "lead-1", "name": "team-lead", "agentType": "lead"},
    {"agentId": "agent-2", "name": "researcher", "agentType": "research"},
    {"agentId": "agent-3", "name": "writer", "agentType": "writer"}
  ]
}`)

	reader := NewReader(homeDir, taskStore)
	status, err := reader.CurrentTeamStatus(ctx)
	if err != nil {
		t.Fatalf("CurrentTeamStatus() error = %v", err)
	}
	if status == nil {
		t.Fatal("CurrentTeamStatus() returned nil status")
	}
	if status.TeamName != "alpha-team" {
		t.Fatalf("TeamName = %q, want alpha-team", status.TeamName)
	}
	if status.LeadAgentID != "lead-1" {
		t.Fatalf("LeadAgentID = %q, want lead-1", status.LeadAgentID)
	}
	if len(status.Members) != 3 {
		t.Fatalf("len(Members) = %d, want 3", len(status.Members))
	}
	if status.Members[0].Status != agent.StatusIdle {
		t.Fatalf("leader status = %q, want idle", status.Members[0].Status)
	}
	if status.Members[1].Status != agent.StatusBusy {
		t.Fatalf("member status = %q, want busy", status.Members[1].Status)
	}
	if got := strings.Join(status.Members[1].CurrentTasks, ","); got != "1,2" {
		t.Fatalf("member current tasks = %q, want 1,2", got)
	}
	if status.Members[2].Status != agent.StatusIdle {
		t.Fatalf("member status = %q, want idle", status.Members[2].Status)
	}
}

// TestCurrentTeamStatus_MissingFile verifies the reader returns nil when no team config exists.
func TestCurrentTeamStatus_MissingFile(t *testing.T) {
	t.Setenv("CLAUDE_CODE_TASK_LIST_ID", "team-missing")

	homeDir := t.TempDir()
	reader := NewReader(homeDir, nil)

	status, err := reader.CurrentTeamStatus(context.Background())
	if err != nil {
		t.Fatalf("CurrentTeamStatus() error = %v", err)
	}
	if status != nil {
		t.Fatalf("CurrentTeamStatus() = %+v, want nil", status)
	}
}
