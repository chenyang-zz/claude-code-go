package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadAgentMemoryPrompt_EmptyMemory(t *testing.T) {
	dir := t.TempDir()
	paths := &Paths{
		CWD:           dir,
		MemoryBaseDir: filepath.Join(dir, "home", ".claude"),
	}

	prompt := LoadAgentMemoryPrompt("explore", ScopeProject, paths)

	if !strings.Contains(prompt, "# Persistent Agent Memory") {
		t.Error("prompt should contain title")
	}
	if !strings.Contains(prompt, "MEMORY.md is currently empty") {
		t.Error("prompt should indicate empty MEMORY.md")
	}
	if !strings.Contains(prompt, "project-scope") {
		t.Error("prompt should contain project scope note")
	}

	// Directory should have been created.
	memDir := paths.GetAgentMemoryDir("explore", ScopeProject)
	if _, err := os.Stat(memDir); os.IsNotExist(err) {
		t.Error("memory directory should have been created")
	}
}

func TestLoadAgentMemoryPrompt_WithContent(t *testing.T) {
	dir := t.TempDir()
	paths := &Paths{
		CWD:           dir,
		MemoryBaseDir: filepath.Join(dir, "home", ".claude"),
	}

	// Write a MEMORY.md file.
	entrypoint := paths.GetAgentMemoryEntrypoint("test-agent", ScopeUser)
	_ = EnsureAgentMemoryDir(filepath.Dir(entrypoint))
	content := "- [Role](role.md) — user is a senior engineer"
	if err := os.WriteFile(entrypoint, []byte(content), 0644); err != nil {
		t.Fatalf("write entrypoint: %v", err)
	}

	prompt := LoadAgentMemoryPrompt("test-agent", ScopeUser, paths)

	if !strings.Contains(prompt, content) {
		t.Error("prompt should contain MEMORY.md content")
	}
	if !strings.Contains(prompt, "user-scope") {
		t.Error("prompt should contain user scope note")
	}
	if !strings.Contains(prompt, "## MEMORY.md") {
		t.Error("prompt should contain MEMORY.md section")
	}
}

func TestLoadAgentMemoryPrompt_LocalScope(t *testing.T) {
	dir := t.TempDir()
	paths := &Paths{
		CWD:           dir,
		MemoryBaseDir: filepath.Join(dir, "home", ".claude"),
	}

	prompt := LoadAgentMemoryPrompt("agent", ScopeLocal, paths)

	if !strings.Contains(prompt, "local-scope") {
		t.Error("prompt should contain local scope note")
	}
}

func TestTruncateEntrypointContent_NoTruncation(t *testing.T) {
	input := "line1\nline2\nline3"
	result := truncateEntrypointContent(input)

	if result.wasLineTruncated {
		t.Error("should not be line-truncated")
	}
	if result.wasByteTruncated {
		t.Error("should not be byte-truncated")
	}
	if result.lineCount != 3 {
		t.Errorf("lineCount = %d, want 3", result.lineCount)
	}
	if !strings.Contains(result.content, "line1") {
		t.Error("content should contain original text")
	}
}

func TestTruncateEntrypointContent_LineTruncation(t *testing.T) {
	lines := make([]string, maxEntrypointLines+10)
	for i := range lines {
		lines[i] = " filler line"
	}
	input := strings.Join(lines, "\n")
	result := truncateEntrypointContent(input)

	if !result.wasLineTruncated {
		t.Error("should be line-truncated")
	}
	if !strings.Contains(result.content, "WARNING") {
		t.Error("truncated content should contain WARNING")
	}
}

func TestTruncateEntrypointContent_ByteTruncation(t *testing.T) {
	// Create a single very long line that exceeds byte cap but not line cap.
	longLine := strings.Repeat("a", maxEntrypointBytes+100)
	input := longLine
	result := truncateEntrypointContent(input)

	if !result.wasByteTruncated {
		t.Error("should be byte-truncated")
	}
	if len(result.content) > maxEntrypointBytes+200 {
		// Allow some slack for the warning message.
		t.Errorf("content length %d exceeds expected max", len(result.content))
	}
	if !strings.Contains(result.content, "WARNING") {
		t.Error("truncated content should contain WARNING")
	}
}

func TestBuildMemoryPrompt_ContainsGuidance(t *testing.T) {
	prompt := buildMemoryPrompt("/tmp/mem", "", "- scope note here")

	expectedParts := []string{
		"# Persistent Agent Memory",
		"/tmp/mem",
		"scope note here",
		"How to save memories",
		"MEMORY.md is currently empty",
	}
	for _, part := range expectedParts {
		if !strings.Contains(prompt, part) {
			t.Errorf("prompt should contain %q", part)
		}
	}
}
