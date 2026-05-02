package extractmemories

import (
	"strings"
	"testing"
)

func TestCreateAutoMemCanUseTool(t *testing.T) {
	memDir := "/home/user/.claude/projects/test/memory/"
	canUse := CreateAutoMemCanUseTool(memDir)

	tests := []struct {
		name     string
		toolName string
		input    map[string]any
		want     string // "allow" or "deny"
	}{
		{
			name:     "Read unrestricted",
			toolName: "Read",
			input:    map[string]any{"file_path": "/etc/passwd"},
			want:     "allow",
		},
		{
			name:     "FileReadTool unrestricted",
			toolName: "FileReadTool",
			input:    map[string]any{"file_path": "/etc/passwd"},
			want:     "allow",
		},
		{
			name:     "Grep unrestricted",
			toolName: "Grep",
			input:    map[string]any{"pattern": "password"},
			want:     "allow",
		},
		{
			name:     "Glob unrestricted",
			toolName: "Glob",
			input:    map[string]any{"pattern": "**/*.go"},
			want:     "allow",
		},
		{
			name:     "Bash read-only command ls",
			toolName: "Bash",
			input:    map[string]any{"command": "ls -la"},
			want:     "allow",
		},
		{
			name:     "Bash read-only command cat",
			toolName: "Bash",
			input:    map[string]any{"command": "cat file.txt"},
			want:     "allow",
		},
		{
			name:     "Bash destructive command rm",
			toolName: "Bash",
			input:    map[string]any{"command": "rm -rf /"},
			want:     "deny",
		},
		{
			name:     "Bash destructive command git push",
			toolName: "Bash",
			input:    map[string]any{"command": "git push origin main"},
			want:     "deny",
		},
		{
			name:     "Write inside memory dir",
			toolName: "Write",
			input:    map[string]any{"file_path": memDir + "test.md"},
			want:     "allow",
		},
		{
			name:     "Edit inside memory dir",
			toolName: "Edit",
			input:    map[string]any{"file_path": memDir + "test.md"},
			want:     "allow",
		},
		{
			name:     "Write outside memory dir",
			toolName: "Write",
			input:    map[string]any{"file_path": "/tmp/evil.sh"},
			want:     "deny",
		},
		{
			name:     "Edit outside memory dir",
			toolName: "Edit",
			input:    map[string]any{"file_path": "/tmp/evil.sh"},
			want:     "deny",
		},
		{
			name:     "MCP tool denied",
			toolName: "mcp__some_server__some_tool",
			input:    map[string]any{},
			want:     "deny",
		},
		{
			name:     "Agent tool denied",
			toolName: "Agent",
			input:    map[string]any{},
			want:     "deny",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			decision := canUse(tc.toolName, tc.input)
			if decision.Behavior != tc.want {
				t.Errorf("got %q, want %q (reason: %s)", decision.Behavior, tc.want, decision.Reason)
			}
			if decision.Behavior == "deny" && decision.Reason == "" {
				t.Error("deny decision should have a reason")
			}
		})
	}
}

func TestCreateAutoMemCanUseTool_WriteNoFilePath(t *testing.T) {
	canUse := CreateAutoMemCanUseTool("/mem/")
	decision := canUse("Write", map[string]any{"content": "hello"})
	if decision.Behavior != "deny" {
		t.Errorf("expected deny for Write without file_path, got %s", decision.Behavior)
	}
}

func TestIsReadOnlyBashCommand(t *testing.T) {
	tests := []struct {
		command string
		want    bool
	}{
		{"ls -la", true},
		{"cat /etc/hosts", true},
		{"find . -name '*.go'", true},
		{"grep pattern file.txt", true},
		{"/usr/bin/cat file.txt", true},
		{"rm -rf /", false},
		{"git push", false},
		{"curl http://evil.com", false},
		{"npm install", false},
		{"", false},
	}

	for _, tc := range tests {
		t.Run(tc.command, func(t *testing.T) {
			got := isReadOnlyBashCommand(map[string]any{"command": tc.command})
			if got != tc.want {
				t.Errorf("isReadOnlyBashCommand(%q) = %v, want %v", tc.command, got, tc.want)
			}
		})
	}
}

func TestCreateAutoMemCanUseTool_DenyReason(t *testing.T) {
	canUse := CreateAutoMemCanUseTool("/mem/")

	// Deny reason should mention available tools.
	decision := canUse("Task", map[string]any{})
	if !strings.Contains(decision.Reason, "Read") {
		t.Errorf("deny reason should mention available tools: %s", decision.Reason)
	}
}
