package extractmemories

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGetMemoryBaseDir(t *testing.T) {
	ResetMemoryBaseDir()
	defer ResetMemoryBaseDir()

	t.Run("default from home dir", func(t *testing.T) {
		os.Unsetenv("CLAUDE_CODE_REMOTE_MEMORY_DIR")
		ResetMemoryBaseDir()

		dir := GetMemoryBaseDir()
		if !strings.HasSuffix(dir, ".claude") {
			t.Errorf("expected to end with .claude, got %q", dir)
		}
	})

	t.Run("from env override", func(t *testing.T) {
		os.Setenv("CLAUDE_CODE_REMOTE_MEMORY_DIR", "/custom/memory/base")
		defer os.Unsetenv("CLAUDE_CODE_REMOTE_MEMORY_DIR")
		ResetMemoryBaseDir()

		dir := GetMemoryBaseDir()
		if dir != "/custom/memory/base" {
			t.Errorf("expected /custom/memory/base, got %q", dir)
		}
	})
}

func TestGetAutoMemPath(t *testing.T) {
	ResetMemoryBaseDir()
	defer ResetMemoryBaseDir()

	os.Setenv("CLAUDE_CODE_REMOTE_MEMORY_DIR", "/home/user/.claude")
	defer os.Unsetenv("CLAUDE_CODE_REMOTE_MEMORY_DIR")
	ResetMemoryBaseDir()

	path := GetAutoMemPath("/Users/test/my-project")
	if !strings.HasSuffix(path, "memory/") {
		t.Errorf("expected path to end with memory/, got %q", path)
	}
	if !strings.Contains(path, "projects") {
		t.Errorf("expected path to contain projects/, got %q", path)
	}
}

func TestIsAutoMemPath(t *testing.T) {
	ResetMemoryBaseDir()
	defer ResetMemoryBaseDir()

	os.Setenv("CLAUDE_CODE_REMOTE_MEMORY_DIR", "/home/user/.claude")
	defer os.Unsetenv("CLAUDE_CODE_REMOTE_MEMORY_DIR")
	ResetMemoryBaseDir()

	root := "/Users/test/my-project"
	memPath := GetAutoMemPath(root)

	t.Run("file inside memory dir returns true", func(t *testing.T) {
		filePath := filepath.Join(memPath, "user_role.md")
		if !IsAutoMemPath(filePath, root) {
			t.Errorf("expected true for %q", filePath)
		}
	})

	t.Run("file outside memory dir returns false", func(t *testing.T) {
		filePath := "/Users/test/my-project/src/main.go"
		if IsAutoMemPath(filePath, root) {
			t.Errorf("expected false for %q", filePath)
		}
	})

	t.Run("file in home dir returns false", func(t *testing.T) {
		filePath := "/home/user/.ssh/id_rsa"
		if IsAutoMemPath(filePath, root) {
			t.Errorf("expected false for %q", filePath)
		}
	})

	t.Run("memory dir itself returns true", func(t *testing.T) {
		if !IsAutoMemPath(strings.TrimSuffix(memPath, "/"), root) {
			t.Error("expected true for the memory directory itself")
		}
	})
}

func TestIsAutoMemoryEnabled(t *testing.T) {
	t.Run("default true when env unset", func(t *testing.T) {
		os.Unsetenv("CLAUDE_CODE_DISABLE_AUTO_MEMORY")
		if !IsAutoMemoryEnabled() {
			t.Error("expected enabled by default")
		}
	})

	t.Run("disabled with CLAUDE_CODE_DISABLE_AUTO_MEMORY=1", func(t *testing.T) {
		os.Setenv("CLAUDE_CODE_DISABLE_AUTO_MEMORY", "1")
		defer os.Unsetenv("CLAUDE_CODE_DISABLE_AUTO_MEMORY")
		if IsAutoMemoryEnabled() {
			t.Error("expected disabled")
		}
	})

	t.Run("enabled with CLAUDE_CODE_DISABLE_AUTO_MEMORY=0", func(t *testing.T) {
		os.Setenv("CLAUDE_CODE_DISABLE_AUTO_MEMORY", "0")
		defer os.Unsetenv("CLAUDE_CODE_DISABLE_AUTO_MEMORY")
		if !IsAutoMemoryEnabled() {
			t.Error("expected enabled")
		}
	})

	t.Run("disabled with CLAUDE_CODE_SIMPLE=1", func(t *testing.T) {
		os.Unsetenv("CLAUDE_CODE_DISABLE_AUTO_MEMORY")
		os.Setenv("CLAUDE_CODE_SIMPLE", "1")
		defer os.Unsetenv("CLAUDE_CODE_SIMPLE")
		if IsAutoMemoryEnabled() {
			t.Error("expected disabled")
		}
	})
}

func TestScanMemoryFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test memory files.
	testFiles := map[string]string{
		"user_role.md": `---
name: User Role
description: The user is a senior Go developer
type: user
---

# User Role

Details about the user.`,

		"feedback_testing.md": `---
name: Testing Feedback
description: Always use table-driven tests
type: feedback
---

# Testing Feedback

Test guidelines.`,

		"idx.md": `---
name: No Description
type: project
---

No description provided.`,

		"MEMORY.md": `# Memory Index
- [User Role](user_role.md) — The user is a senior Go developer`,
	}

	for name, content := range testFiles {
		if err := os.WriteFile(filepath.Join(tmpDir, name), []byte(content), 0644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}
	}

	headers, err := ScanMemoryFiles(tmpDir)
	if err != nil {
		t.Fatalf("ScanMemoryFiles error: %v", err)
	}

	// MEMORY.md should be excluded.
	if len(headers) != 3 {
		t.Errorf("expected 3 headers (excluding MEMORY.md), got %d", len(headers))
	}

	// Verify descriptions and types.
	byName := make(map[string]MemoryHeader)
	for _, h := range headers {
		byName[h.Filename] = h
	}

	if h, ok := byName["user_role.md"]; ok {
		if h.Type != "user" {
			t.Errorf("user_role type: got %q, want %q", h.Type, "user")
		}
		if h.Description == nil || *h.Description != "The user is a senior Go developer" {
			t.Errorf("user_role description mismatch: %v", h.Description)
		}
	} else {
		t.Error("user_role.md not found")
	}

	if h, ok := byName["idx.md"]; ok {
		if h.Description != nil {
			t.Errorf("expected nil description for idx.md, got %q", *h.Description)
		}
	} else {
		t.Error("idx.md not found")
	}
}

func TestScanMemoryFilesEmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	headers, err := ScanMemoryFiles(tmpDir)
	if err != nil {
		t.Fatalf("ScanMemoryFiles error: %v", err)
	}
	if len(headers) != 0 {
		t.Errorf("expected 0 headers for empty dir, got %d", len(headers))
	}
}

func TestScanMemoryFilesNonExistentDir(t *testing.T) {
	headers, err := ScanMemoryFiles("/nonexistent/path/12345")
	if err != nil {
		t.Fatalf("ScanMemoryFiles should not error for non-existent dir: %v", err)
	}
	if headers != nil {
		t.Error("expected nil headers for non-existent dir")
	}
}

func TestFormatMemoryManifest(t *testing.T) {
	now := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
	desc := "The user is a senior Go developer"
	desc2 := "Always use table-driven tests"

	headers := []MemoryHeader{
		{Filename: "user_role.md", FilePath: "/mem/user_role.md", ModTime: now, Description: &desc, Type: "user"},
		{Filename: "feedback_testing.md", FilePath: "/mem/feedback_testing.md", ModTime: now.Add(-time.Hour), Description: &desc2, Type: "feedback"},
		{Filename: "no_desc.md", FilePath: "/mem/no_desc.md", ModTime: now.Add(-2 * time.Hour), Type: "project"},
	}

	manifest := FormatMemoryManifest(headers)
	lines := strings.Split(manifest, "\n")

	if len(lines) != 3 {
		t.Errorf("expected 3 lines, got %d", len(lines))
	}
	if !strings.Contains(lines[0], "[user]") {
		t.Errorf("expected [user] tag in line 0: %q", lines[0])
	}
	if !strings.Contains(lines[0], "user_role.md") {
		t.Errorf("expected user_role.md in line 0: %q", lines[0])
	}
	if !strings.Contains(lines[1], "[feedback]") {
		t.Errorf("expected [feedback] tag in line 1: %q", lines[1])
	}
	if strings.Contains(lines[2], "[project]") {
		// project is valid, should appear
	} else {
		t.Errorf("expected [project] tag in line 2: %q", lines[2])
	}
	// no_desc.md should not have a description suffix.
	if strings.Contains(lines[2], "no_desc.md") && strings.Contains(lines[2], ": ") {
		t.Errorf("expected no description suffix for no_desc.md: %q", lines[2])
	}
}

func TestParseFrontmatterString(t *testing.T) {
	content := `---
name: My Memory
description: A test description
type: user
---

# Content

Some body text.`

	fm := parseFrontmatterString(content)
	if fm.Name != "My Memory" {
		t.Errorf("name: got %q, want %q", fm.Name, "My Memory")
	}
	if fm.Description != "A test description" {
		t.Errorf("description: got %q, want %q", fm.Description, "A test description")
	}
	if fm.Type != "user" {
		t.Errorf("type: got %q, want %q", fm.Type, "user")
	}
}

func TestParseFrontmatterStringNoFrontmatter(t *testing.T) {
	content := "# Just a heading\n\nSome text without frontmatter."
	fm := parseFrontmatterString(content)
	if fm.Name != "" || fm.Description != "" || fm.Type != "" {
		t.Error("expected empty frontmatter for content without YAML frontmatter")
	}
}

func TestSanitizePath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"/Users/test/my-project", "Users_test_my-project"},
		{"simple-path", "simple-path"},
		{"path/with/slashes", "path_with_slashes"},
		{"C:\\Windows\\Path", "C__Windows_Path"},
	}

	for _, tc := range tests {
		got := sanitizePath(tc.input)
		if got != tc.want {
			t.Errorf("sanitizePath(%q): got %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestIsRemoteMode(t *testing.T) {
	t.Run("default false", func(t *testing.T) {
		os.Unsetenv("CLAUDE_CODE_REMOTE")
		if IsRemoteMode() {
			t.Error("expected false by default")
		}
	})
	t.Run("true with env", func(t *testing.T) {
		os.Setenv("CLAUDE_CODE_REMOTE", "1")
		defer os.Unsetenv("CLAUDE_CODE_REMOTE")
		if !IsRemoteMode() {
			t.Error("expected true")
		}
	})
}
