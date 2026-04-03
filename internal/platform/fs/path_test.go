package fs

import (
	"os"
	"path/filepath"
	"testing"
)

// TestExpandPathResolvesRelativePath verifies relative inputs are resolved from the provided base directory.
func TestExpandPathResolvesRelativePath(t *testing.T) {
	got, err := ExpandPath("./src/../docs", "/workspace/project")
	if err != nil {
		t.Fatalf("ExpandPath() error = %v", err)
	}

	want := filepath.Clean("/workspace/project/docs")
	if got != want {
		t.Fatalf("ExpandPath() = %q, want %q", got, want)
	}
}

// TestExpandPathExpandsHome verifies tilde-prefixed paths are expanded to the user home directory.
func TestExpandPathExpandsHome(t *testing.T) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("os.UserHomeDir() error = %v", err)
	}

	got, err := ExpandPath("~/notes", "/workspace/project")
	if err != nil {
		t.Fatalf("ExpandPath() error = %v", err)
	}

	want := filepath.Join(homeDir, "notes")
	if got != want {
		t.Fatalf("ExpandPath() = %q, want %q", got, want)
	}
}

// TestExpandPathUsesBaseDirForWhitespaceInput verifies blank inputs collapse to the normalized base directory.
func TestExpandPathUsesBaseDirForWhitespaceInput(t *testing.T) {
	got, err := ExpandPath("   ", "/workspace/project/./subdir/..")
	if err != nil {
		t.Fatalf("ExpandPath() error = %v", err)
	}

	want := filepath.Clean("/workspace/project")
	if got != want {
		t.Fatalf("ExpandPath() = %q, want %q", got, want)
	}
}

// TestExpandPathRejectsNullBytes verifies path validation rejects unsafe null-byte input.
func TestExpandPathRejectsNullBytes(t *testing.T) {
	if _, err := ExpandPath("bad\x00path", "/workspace/project"); err == nil {
		t.Fatal("ExpandPath() error = nil, want non-nil")
	}
}

// TestToRelativePathKeepsPathsInsideWorkingDir verifies in-workspace files are shortened for tool output.
func TestToRelativePathKeepsPathsInsideWorkingDir(t *testing.T) {
	got := ToRelativePath("/workspace/project/docs/readme.md", "/workspace/project")
	if got != filepath.Join("docs", "readme.md") {
		t.Fatalf("ToRelativePath() = %q, want %q", got, filepath.Join("docs", "readme.md"))
	}
}

// TestToRelativePathKeepsAbsolutePathsOutsideWorkingDir verifies paths outside the workspace remain unambiguous.
func TestToRelativePathKeepsAbsolutePathsOutsideWorkingDir(t *testing.T) {
	got := ToRelativePath("/workspace/other/readme.md", "/workspace/project")
	if got != "/workspace/other/readme.md" {
		t.Fatalf("ToRelativePath() = %q, want %q", got, "/workspace/other/readme.md")
	}
}
