package team

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSanitizeName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"my-team", "my-team"},
		{"My Team", "my-team"},
		{"team@name", "team-name"},
		{"UPPERCASE", "uppercase"},
		{"team_name!", "team-name-"},
		{"123team", "123team"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := SanitizeName(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestTeamDir(t *testing.T) {
	dir := TeamDir("/home/user", "my-team")
	expected := filepath.Join("/home/user", ".claude", "teams", "my-team")
	if dir != expected {
		t.Errorf("TeamDir = %q, want %q", dir, expected)
	}
}

func TestTeamDir_SanitizesName(t *testing.T) {
	dir := TeamDir("/home/user", "My Team@Work!")
	// "My Team@Work!" → "my-team-work-"
	expected := filepath.Join("/home/user", ".claude", "teams", "my-team-work-")
	if dir != expected {
		t.Errorf("TeamDir = %q, want %q", dir, expected)
	}
}

func TestTeamFilePath(t *testing.T) {
	path := TeamFilePath("/home/user", "alpha")
	expected := filepath.Join("/home/user", ".claude", "teams", "alpha", "config.json")
	if path != expected {
		t.Errorf("TeamFilePath = %q, want %q", path, expected)
	}
}

func TestTaskDir(t *testing.T) {
	dir := TaskDir("/home/user", "my-team")
	expected := filepath.Join("/home/user", ".claude", "tasks", "my-team")
	if dir != expected {
		t.Errorf("TaskDir = %q, want %q", dir, expected)
	}
}

func TestWriteTeamFile(t *testing.T) {
	dir := t.TempDir()
	homeDir := filepath.Join(dir, "home")

	teamFile := &TeamFile{
		Name:        "test-team",
		Description: "A test team",
		CreatedAt:   time.Now().UnixMilli(),
		LeadAgentID: "team-lead@test-team",
		Members: []TeamMember{
			{AgentID: "team-lead@test-team", Name: "team-lead", AgentType: "team-lead"},
		},
	}

	err := WriteTeamFile(homeDir, "test-team", teamFile)
	if err != nil {
		t.Fatalf("WriteTeamFile failed: %v", err)
	}

	// Verify the file was written
	path := TeamFilePath(homeDir, "test-team")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	var readBack TeamFile
	if err := json.Unmarshal(data, &readBack); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if readBack.Name != "test-team" {
		t.Errorf("Name = %q, want %q", readBack.Name, "test-team")
	}
	if readBack.Description != "A test team" {
		t.Errorf("Description = %q, want %q", readBack.Description, "A test team")
	}
	if readBack.LeadAgentID != "team-lead@test-team" {
		t.Errorf("LeadAgentID = %q, want %q", readBack.LeadAgentID, "team-lead@test-team")
	}
	if len(readBack.Members) != 1 {
		t.Fatalf("Members len = %d, want 1", len(readBack.Members))
	}
	if readBack.Members[0].Name != "team-lead" {
		t.Errorf("Members[0].Name = %q, want %q", readBack.Members[0].Name, "team-lead")
	}
}

func TestWriteTeamFile_Overwrite(t *testing.T) {
	dir := t.TempDir()
	homeDir := filepath.Join(dir, "home")

	first := &TeamFile{
		Name:        "test-team",
		LeadAgentID: "lead-1",
		Members:     []TeamMember{{AgentID: "lead-1", Name: "team-lead"}},
	}
	if err := WriteTeamFile(homeDir, "test-team", first); err != nil {
		t.Fatalf("first WriteTeamFile failed: %v", err)
	}

	second := &TeamFile{
		Name:        "test-team",
		Description: "Updated",
		LeadAgentID: "lead-2",
		Members:     []TeamMember{{AgentID: "lead-2", Name: "team-lead", AgentType: "researcher"}},
	}
	if err := WriteTeamFile(homeDir, "test-team", second); err != nil {
		t.Fatalf("second WriteTeamFile failed: %v", err)
	}

	readBack, err := ReadTeamFile(homeDir, "test-team")
	if err != nil {
		t.Fatalf("ReadTeamFile failed: %v", err)
	}
	if readBack.LeadAgentID != "lead-2" {
		t.Errorf("LeadAgentID = %q, want %q", readBack.LeadAgentID, "lead-2")
	}
	if readBack.Description != "Updated" {
		t.Errorf("Description = %q, want %q", readBack.Description, "Updated")
	}
}

func TestReadTeamFile(t *testing.T) {
	dir := t.TempDir()
	homeDir := filepath.Join(dir, "home")

	teamFile := &TeamFile{
		Name:        "read-test",
		LeadAgentID: "lead@read-test",
		Members:     []TeamMember{{AgentID: "lead@read-test", Name: "team-lead"}},
	}
	if err := WriteTeamFile(homeDir, "read-test", teamFile); err != nil {
		t.Fatalf("WriteTeamFile failed: %v", err)
	}

	readBack, err := ReadTeamFile(homeDir, "read-test")
	if err != nil {
		t.Fatalf("ReadTeamFile failed: %v", err)
	}
	if readBack == nil {
		t.Fatal("ReadTeamFile returned nil")
	}
	if readBack.Name != "read-test" {
		t.Errorf("Name = %q, want %q", readBack.Name, "read-test")
	}
}

func TestReadTeamFile_NonExistent(t *testing.T) {
	dir := t.TempDir()
	readBack, err := ReadTeamFile(dir, "nonexistent")
	if err != nil {
		t.Fatalf("ReadTeamFile returned error: %v", err)
	}
	if readBack != nil {
		t.Error("ReadTeamFile should return nil for non-existent team")
	}
}

func TestDeleteTeamDirectories(t *testing.T) {
	dir := t.TempDir()
	homeDir := filepath.Join(dir, "home")

	// Create team file and task directory
	teamFile := &TeamFile{
		Name:        "delete-test",
		LeadAgentID: "lead@delete-test",
		Members:     []TeamMember{{AgentID: "lead@delete-test", Name: "team-lead"}},
	}
	if err := WriteTeamFile(homeDir, "delete-test", teamFile); err != nil {
		t.Fatalf("WriteTeamFile failed: %v", err)
	}

	// Create a dummy task directory
	taskDir := TaskDir(homeDir, "delete-test")
	if err := os.MkdirAll(taskDir, 0o755); err != nil {
		t.Fatalf("MkdirAll task dir failed: %v", err)
	}
	// Write a dummy file in task dir
	if err := os.WriteFile(filepath.Join(taskDir, "test.txt"), []byte("data"), 0o644); err != nil {
		t.Fatalf("WriteFile task file failed: %v", err)
	}

	DeleteTeamDirectories(homeDir, "delete-test")

	// Verify team directory is gone
	teamPath := TeamFilePath(homeDir, "delete-test")
	if _, err := os.Stat(teamPath); !os.IsNotExist(err) {
		t.Error("team config file should be deleted")
	}

	// Verify task directory is gone
	if _, err := os.Stat(taskDir); !os.IsNotExist(err) {
		t.Error("task directory should be deleted")
	}
}

func TestDeleteTeamDirectories_Idempotent(t *testing.T) {
	dir := t.TempDir()
	homeDir := filepath.Join(dir, "home")

	// Delete a non-existent team should not error
	DeleteTeamDirectories(homeDir, "never-existed")
	// Test passes if no panic
}
