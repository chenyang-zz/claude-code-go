package sessionmemory_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/services/sessionmemory"
)

func TestGetSessionMemoryDir(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	dir := sessionmemory.GetSessionMemoryDir()
	if !strings.HasSuffix(dir, ".claude/session-memory/") {
		t.Errorf("GetSessionMemoryDir() = %q, want suffix %q", dir, ".claude/session-memory/")
	}
}

func TestGetSessionMemoryPath(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	path := sessionmemory.GetSessionMemoryPath()
	if !strings.HasSuffix(path, "summary.md") {
		t.Errorf("GetSessionMemoryPath() = %q, want suffix %q", path, "summary.md")
	}
}

func TestSetupSessionMemoryFile_CreatesNewFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	ctx := context.Background()
	memoryPath, content, err := sessionmemory.SetupSessionMemoryFile(ctx)
	if err != nil {
		t.Fatalf("SetupSessionMemoryFile() error = %v", err)
	}

	// File should exist on disk.
	if _, statErr := os.Stat(memoryPath); os.IsNotExist(statErr) {
		t.Fatalf("file was not created at %s", memoryPath)
	}

	// Content should be the default template.
	if content != sessionmemory.DefaultSessionMemoryTemplate {
		t.Errorf("file content doesn't match DefaultSessionMemoryTemplate")
	}

	// Read directly from disk to double-check.
	raw, _ := os.ReadFile(memoryPath)
	if string(raw) != sessionmemory.DefaultSessionMemoryTemplate {
		t.Errorf("on-disk content doesn't match DefaultSessionMemoryTemplate")
	}
}

func TestSetupSessionMemoryFile_ExistingFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Create the file with custom content first.
	memoryPath := filepath.Join(tmpDir, ".claude", "session-memory", "summary.md")
	os.MkdirAll(filepath.Dir(memoryPath), 0700)
	customContent := "# Custom content"
	if err := os.WriteFile(memoryPath, []byte(customContent), 0600); err != nil {
		t.Fatalf("failed to create pre-existing file: %v", err)
	}

	ctx := context.Background()
	_, content, err := sessionmemory.SetupSessionMemoryFile(ctx)
	if err != nil {
		t.Fatalf("SetupSessionMemoryFile() error = %v", err)
	}

	// Content must remain unchanged.
	if content != customContent {
		t.Errorf("got content = %q, want %q", content, customContent)
	}
}

func TestGetSessionMemoryContent_FileExists(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Create the file with known content.
	memoryPath := filepath.Join(tmpDir, ".claude", "session-memory", "summary.md")
	os.MkdirAll(filepath.Dir(memoryPath), 0700)
	want := "# Some session memory content"
	if err := os.WriteFile(memoryPath, []byte(want), 0600); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	ctx := context.Background()
	got, err := sessionmemory.GetSessionMemoryContent(ctx)
	if err != nil {
		t.Fatalf("GetSessionMemoryContent() error = %v", err)
	}
	if got != want {
		t.Errorf("GetSessionMemoryContent() = %q, want %q", got, want)
	}
}

func TestGetSessionMemoryContent_FileNotExists(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	ctx := context.Background()
	got, err := sessionmemory.GetSessionMemoryContent(ctx)
	if err != nil {
		t.Fatalf("GetSessionMemoryContent() error = %v", err)
	}
	if got != "" {
		t.Errorf("GetSessionMemoryContent() = %q, want empty string", got)
	}
}
