package fs

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLocalFSReadWrite verifies the local filesystem adapter can create, read, stat, rename, and remove files.
func TestLocalFSReadWrite(t *testing.T) {
	t.Parallel()

	filesystem := NewLocalFS()
	tempDir := t.TempDir()

	originalPath := filepath.Join(tempDir, "nested", "input.txt")
	renamedPath := filepath.Join(tempDir, "nested", "output.txt")

	if err := filesystem.MkdirAll(filepath.Dir(originalPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	if err := filesystem.WriteFile(originalPath, []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	info, err := filesystem.Stat(originalPath)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if info.Size() != 5 {
		t.Fatalf("Stat() size = %d, want %d", info.Size(), 5)
	}

	data, err := filesystem.ReadFile(originalPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("ReadFile() data = %q, want %q", string(data), "hello")
	}

	if err := filesystem.Rename(originalPath, renamedPath); err != nil {
		t.Fatalf("Rename() error = %v", err)
	}

	if _, err := filesystem.Stat(originalPath); !IsNotExist(err) {
		t.Fatalf("Stat() after Rename() error = %v, want not-exist", err)
	}

	if err := filesystem.Remove(renamedPath); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}

	if _, err := filesystem.Stat(renamedPath); !IsNotExist(err) {
		t.Fatalf("Stat() after Remove() error = %v, want not-exist", err)
	}
}

// TestLocalFSReadDir verifies the adapter returns directory entries from the local filesystem.
func TestLocalFSReadDir(t *testing.T) {
	t.Parallel()

	filesystem := NewLocalFS()
	tempDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(tempDir, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "b.txt"), []byte("b"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	entries, err := filesystem.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("ReadDir() len = %d, want %d", len(entries), 2)
	}
}
