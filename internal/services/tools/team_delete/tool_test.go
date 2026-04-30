package team_delete

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	"github.com/sheepzhao/claude-code-go/internal/platform/team"
)

func newTestTool(t *testing.T) *Tool {
	t.Helper()
	return NewTool(t.TempDir())
}

func TestName(t *testing.T) {
	tool := newTestTool(t)
	if tool.Name() != Name {
		t.Errorf("Name() = %q, want %q", tool.Name(), Name)
	}
}

func TestDescription(t *testing.T) {
	tool := newTestTool(t)
	if tool.Description() == "" {
		t.Error("Description() should not be empty")
	}
}

func TestInputSchema(t *testing.T) {
	tool := newTestTool(t)
	schema := tool.InputSchema()

	if schema.Properties["team_name"].Type != coretool.ValueKindString {
		t.Error("team_name should be string type")
	}
	if !schema.Properties["team_name"].Required {
		t.Error("team_name should be required")
	}
}

func TestIsReadOnly(t *testing.T) {
	tool := newTestTool(t)
	if tool.IsReadOnly() {
		t.Error("TeamDelete should not be read-only")
	}
}

func TestIsConcurrencySafe(t *testing.T) {
	tool := newTestTool(t)
	if tool.IsConcurrencySafe() {
		t.Error("TeamDelete should not be concurrency-safe")
	}
}

func TestInvoke_DeleteExistingTeam(t *testing.T) {
	homeDir := t.TempDir()

	// First create a team
	teamFile := &team.TeamFile{
		Name:        "delete-me",
		LeadAgentID: "team-lead@delete-me",
		Members:     []team.TeamMember{{AgentID: "team-lead@delete-me", Name: "team-lead"}},
	}
	if err := team.WriteTeamFile(homeDir, "delete-me", teamFile); err != nil {
		t.Fatalf("WriteTeamFile failed: %v", err)
	}

	tool := NewTool(homeDir)
	ctx := context.Background()

	result, err := tool.Invoke(ctx, coretool.Call{
		Input: map[string]any{
			"team_name": "delete-me",
		},
	})
	if err != nil {
		t.Fatalf("Invoke failed: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected error: %s", result.Error)
	}

	var out Output
	if err := json.Unmarshal([]byte(result.Output), &out); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}
	if !out.Success {
		t.Error("expected success=true")
	}
	if out.TeamName != "delete-me" {
		t.Errorf("TeamName = %q, want %q", out.TeamName, "delete-me")
	}

	// Verify the team file is gone
	readBack, _ := team.ReadTeamFile(homeDir, "delete-me")
	if readBack != nil {
		t.Error("team file should be deleted")
	}
}

func TestInvoke_DeleteNonExistentTeam(t *testing.T) {
	tool := newTestTool(t)
	ctx := context.Background()

	result, err := tool.Invoke(ctx, coretool.Call{
		Input: map[string]any{
			"team_name": "never-existed",
		},
	})
	if err != nil {
		t.Fatalf("Invoke failed: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected error: %s", result.Error)
	}

	var out Output
	json.Unmarshal([]byte(result.Output), &out)
	if !out.Success {
		t.Error("deleting non-existent team should still return success=true")
	}
}

func TestInvoke_EmptyTeamName(t *testing.T) {
	tool := newTestTool(t)
	ctx := context.Background()

	result, err := tool.Invoke(ctx, coretool.Call{
		Input: map[string]any{
			"team_name": "   ",
		},
	})
	if err != nil {
		t.Fatalf("Invoke failed: %v", err)
	}
	if result.Error == "" {
		t.Error("expected error for empty team_name")
	}
}

func TestInvoke_MissingTeamName(t *testing.T) {
	tool := newTestTool(t)
	ctx := context.Background()

	result, err := tool.Invoke(ctx, coretool.Call{
		Input: map[string]any{},
	})
	if err != nil {
		t.Fatalf("Invoke failed: %v", err)
	}
	if result.Error == "" {
		t.Error("expected error for missing team_name")
	}
}

func TestInvoke_DeletesTaskDirectory(t *testing.T) {
	homeDir := t.TempDir()

	// Create team with a task directory containing files
	teamFile := &team.TeamFile{
		Name:        "full-cleanup",
		LeadAgentID: "lead@full-cleanup",
		Members:     []team.TeamMember{{AgentID: "lead@full-cleanup", Name: "team-lead"}},
	}
	if err := team.WriteTeamFile(homeDir, "full-cleanup", teamFile); err != nil {
		t.Fatalf("WriteTeamFile failed: %v", err)
	}

	taskDir := team.TaskDir(homeDir, "full-cleanup")
	if err := os.MkdirAll(taskDir, 0o755); err != nil {
		t.Fatalf("MkdirAll task dir failed: %v", err)
	}

	tool := NewTool(homeDir)
	ctx := context.Background()

	result, err := tool.Invoke(ctx, coretool.Call{
		Input: map[string]any{"team_name": "full-cleanup"},
	})
	if err != nil {
		t.Fatalf("Invoke failed: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected error: %s", result.Error)
	}

	// Verify task directory is gone
	if _, err := os.Stat(taskDir); !os.IsNotExist(err) {
		t.Error("task directory should be deleted")
	}
}
