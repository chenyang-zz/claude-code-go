package shared

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestCheckTeamMemorySecretsRejectsSecretsInTeamMemory verifies team memory content with secrets is rejected.
func TestCheckTeamMemorySecretsRejectsSecretsInTeamMemory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "projects", "demo", "memory", "team", "MEMORY.md")
	content := "ghp_" + strings.Repeat("a", 36) + "\n" + "xoxb-1234567890-1234567890abcde"

	message := CheckTeamMemorySecrets(path, content)
	if message == "" {
		t.Fatalf("CheckTeamMemorySecrets() = %q, want rejection message", message)
	}
	if !strings.Contains(message, "GitHub PAT") {
		t.Fatalf("CheckTeamMemorySecrets() = %q, want GitHub PAT label", message)
	}
	if !strings.Contains(message, "Slack bot token") {
		t.Fatalf("CheckTeamMemorySecrets() = %q, want Slack bot token label", message)
	}
}

// TestCheckTeamMemorySecretsIgnoresNonTeamMemoryPaths verifies non-team-memory files are not blocked.
func TestCheckTeamMemorySecretsIgnoresNonTeamMemoryPaths(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.md")
	content := "ghp_" + strings.Repeat("a", 36)

	message := CheckTeamMemorySecrets(path, content)
	if message != "" {
		t.Fatalf("CheckTeamMemorySecrets() = %q, want empty string", message)
	}
}

// TestIsTeamMemoryPath detects the team-memory directory layout used by the guard.
func TestIsTeamMemoryPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "projects", "demo", "memory", "team", "MEMORY.md")
	if !IsTeamMemoryPath(path) {
		t.Fatalf("IsTeamMemoryPath(%q) = false, want true", path)
	}
}
