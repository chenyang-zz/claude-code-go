package team_create

import (
	"context"
	"encoding/json"
	"path/filepath"
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
	if _, ok := schema.Properties["description"]; !ok {
		t.Error("description should be in schema")
	}
	if _, ok := schema.Properties["agent_type"]; !ok {
		t.Error("agent_type should be in schema")
	}
}

func TestIsReadOnly(t *testing.T) {
	tool := newTestTool(t)
	if tool.IsReadOnly() {
		t.Error("TeamCreate should not be read-only")
	}
}

func TestIsConcurrencySafe(t *testing.T) {
	tool := newTestTool(t)
	if tool.IsConcurrencySafe() {
		t.Error("TeamCreate should not be concurrency-safe")
	}
}

func TestInvoke_CreateTeam(t *testing.T) {
	tool := newTestTool(t)
	ctx := context.Background()

	result, err := tool.Invoke(ctx, coretool.Call{
		Input: map[string]any{
			"team_name":   "my-test-team",
			"description": "A test swarm",
			"agent_type":  "researcher",
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
	if out.TeamName != "my-test-team" {
		t.Errorf("TeamName = %q, want %q", out.TeamName, "my-test-team")
	}
	if out.LeadAgentID != "team-lead@my-test-team" {
		t.Errorf("LeadAgentID = %q, want %q", out.LeadAgentID, "team-lead@my-test-team")
	}

	expectedPath := team.TeamFilePath(tool.homeDir, "my-test-team")
	if out.TeamFilePath != expectedPath {
		t.Errorf("TeamFilePath = %q, want %q", out.TeamFilePath, expectedPath)
	}

	// Verify the file was actually written
	readBack, err := team.ReadTeamFile(tool.homeDir, "my-test-team")
	if err != nil {
		t.Fatalf("ReadTeamFile failed: %v", err)
	}
	if readBack == nil {
		t.Fatal("team file should exist after create")
	}
	if readBack.Name != "my-test-team" {
		t.Errorf("Name = %q, want %q", readBack.Name, "my-test-team")
	}
	if readBack.Description != "A test swarm" {
		t.Errorf("Description = %q, want %q", readBack.Description, "A test swarm")
	}
	if len(readBack.Members) != 1 {
		t.Fatalf("Members len = %d, want 1", len(readBack.Members))
	}
	if readBack.Members[0].Name != "team-lead" {
		t.Errorf("Member name = %q, want %q", readBack.Members[0].Name, "team-lead")
	}
}

func TestInvoke_DefaultAgentType(t *testing.T) {
	tool := newTestTool(t)
	ctx := context.Background()

	result, err := tool.Invoke(ctx, coretool.Call{
		Input: map[string]any{
			"team_name": "minimal-team",
		},
	})
	if err != nil {
		t.Fatalf("Invoke failed: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected error: %s", result.Error)
	}

	readBack, _ := team.ReadTeamFile(tool.homeDir, "minimal-team")
	if readBack.Members[0].AgentType != "team-lead" {
		t.Errorf("AgentType = %q, want %q", readBack.Members[0].AgentType, "team-lead")
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

func TestInvoke_DuplicateTeam(t *testing.T) {
	tool := newTestTool(t)
	ctx := context.Background()

	// First create succeeds
	result, err := tool.Invoke(ctx, coretool.Call{
		Input: map[string]any{
			"team_name": "duplicate-team",
		},
	})
	if err != nil {
		t.Fatalf("first Invoke failed: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("first create should succeed, got: %s", result.Error)
	}

	// Second create with same name fails
	result, err = tool.Invoke(ctx, coretool.Call{
		Input: map[string]any{
			"team_name": "duplicate-team",
		},
	})
	if err != nil {
		t.Fatalf("second Invoke failed: %v", err)
	}
	if result.Error == "" {
		t.Error("expected error for duplicate team_name")
	}
}

func TestInvoke_SanitizedPath(t *testing.T) {
	tool := newTestTool(t)
	ctx := context.Background()

	result, err := tool.Invoke(ctx, coretool.Call{
		Input: map[string]any{
			"team_name": "My Team@Work",
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

	// Path should use sanitized name
	sanitized := team.SanitizeName("My Team@Work")
	expectedPath := filepath.Join(tool.homeDir, ".claude", "teams", sanitized, "config.json")
	if out.TeamFilePath != expectedPath {
		t.Errorf("TeamFilePath = %q, want %q", out.TeamFilePath, expectedPath)
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
