package memory

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSanitizeAgentTypeForPath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"my-agent", "my-agent"},
		{"plugin:my-agent", "plugin-my-agent"},
		{"a:b:c", "a-b-c"},
		{"explore", "explore"},
	}
	for _, tc := range tests {
		got := sanitizeAgentTypeForPath(tc.input)
		if got != tc.expected {
			t.Errorf("sanitizeAgentTypeForPath(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestPaths_GetAgentMemoryDir(t *testing.T) {
	p := &Paths{
		CWD:           "/project",
		MemoryBaseDir: "/home/user/.claude",
	}

	tests := []struct {
		agentType string
		scope     AgentMemoryScope
		want      string
	}{
		{
			agentType: "explore",
			scope:     ScopeUser,
			want:      filepath.Join("/home/user/.claude", "agent-memory", "explore") + string(filepath.Separator),
		},
		{
			agentType: "test-runner",
			scope:     ScopeProject,
			want:      filepath.Join("/project", ".claude", "agent-memory", "test-runner") + string(filepath.Separator),
		},
		{
			agentType: "custom-agent",
			scope:     ScopeLocal,
			want:      filepath.Join("/project", ".claude", "agent-memory-local", "custom-agent") + string(filepath.Separator),
		},
		{
			agentType: "plugin:agent",
			scope:     ScopeProject,
			want:      filepath.Join("/project", ".claude", "agent-memory", "plugin-agent") + string(filepath.Separator),
		},
	}

	for _, tc := range tests {
		got := p.GetAgentMemoryDir(tc.agentType, tc.scope)
		if got != tc.want {
			t.Errorf("GetAgentMemoryDir(%q, %q) = %q, want %q", tc.agentType, tc.scope, got, tc.want)
		}
	}
}

func TestPaths_GetAgentMemoryEntrypoint(t *testing.T) {
	p := &Paths{
		CWD:           "/project",
		MemoryBaseDir: "/home/user/.claude",
	}

	got := p.GetAgentMemoryEntrypoint("explore", ScopeProject)
	want := filepath.Join("/project", ".claude", "agent-memory", "explore", "MEMORY.md")
	if got != want {
		t.Errorf("GetAgentMemoryEntrypoint = %q, want %q", got, want)
	}
}

func TestPaths_IsAgentMemoryPath(t *testing.T) {
	p := &Paths{
		CWD:           "/project",
		MemoryBaseDir: "/home/user/.claude",
	}

	tests := []struct {
		path string
		want bool
	}{
		// User scope.
		{filepath.Join("/home/user/.claude", "agent-memory", "explore", "MEMORY.md"), true},
		{filepath.Join("/home/user/.claude", "agent-memory", "foo", "bar.md"), true},
		// Project scope.
		{filepath.Join("/project", ".claude", "agent-memory", "test", "MEMORY.md"), true},
		{filepath.Join("/project", ".claude", "agent-memory", "x", "y.md"), true},
		// Local scope.
		{filepath.Join("/project", ".claude", "agent-memory-local", "a", "b.md"), true},
		// Not agent memory.
		{filepath.Join("/project", ".claude", "settings.json"), false},
		{filepath.Join("/home/user/.claude", "memory", "foo.md"), false},
		{"/etc/passwd", false},
		// Traversal attempt should be normalized and fail.
		{filepath.Join("/project", ".claude", "agent-memory", "..", "..", "secret.txt"), false},
	}

	for _, tc := range tests {
		got := p.IsAgentMemoryPath(tc.path)
		if got != tc.want {
			t.Errorf("IsAgentMemoryPath(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestEnsureAgentMemoryDir(t *testing.T) {
	dir := t.TempDir()
	memDir := filepath.Join(dir, "a", "b", "c")

	if _, err := os.Stat(memDir); !os.IsNotExist(err) {
		t.Fatal("directory should not exist yet")
	}

	if err := EnsureAgentMemoryDir(memDir); err != nil {
		t.Fatalf("EnsureAgentMemoryDir failed: %v", err)
	}

	info, err := os.Stat(memDir)
	if err != nil {
		t.Fatalf("stat failed: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("expected directory")
	}

	// Idempotent — should not error on second call.
	if err := EnsureAgentMemoryDir(memDir); err != nil {
		t.Fatalf("second EnsureAgentMemoryDir failed: %v", err)
	}
}

func TestPaths_Defaults(t *testing.T) {
	p := &Paths{}
	cwd := p.cwd()
	if cwd == "" {
		t.Error("expected cwd to default to os.Getwd()")
	}
	base := p.memoryBaseDir()
	if base == "" {
		t.Error("expected memoryBaseDir to default to ~/.claude")
	}
}
