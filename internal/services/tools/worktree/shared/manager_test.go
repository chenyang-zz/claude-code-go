package shared

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestValidateSlugValid(t *testing.T) {
	m := NewManager()
	tests := []string{
		"feature",
		"my-feature",
		"bugfix/123",
		"user.name",
		"a.b_c-d",
		"x",
		"abc123",
	}
	for _, slug := range tests {
		t.Run(slug, func(t *testing.T) {
			if err := m.ValidateSlug(slug); err != nil {
				t.Errorf("ValidateSlug(%q) unexpected error: %v", slug, err)
			}
		})
	}
}

func TestValidateSlugInvalid(t *testing.T) {
	m := NewManager()
	tests := []struct {
		slug string
		msg  string
	}{
		{"", "empty segment"},
		{".", "dot"},
		{"..", "dotdot"},
		{"feature/..", "path traversal"},
		{"feature/.", "hidden segment"},
		{"has space", "space"},
		{"feature?", "question mark"},
	}
	for _, tc := range tests {
		t.Run(tc.slug, func(t *testing.T) {
			if err := m.ValidateSlug(tc.slug); err == nil {
				t.Errorf("ValidateSlug(%q) expected error for %s", tc.slug, tc.msg)
			}
		})
	}
}

func TestValidateSlugMaxLength(t *testing.T) {
	m := NewManager()
	slug := ""
	for i := 0; i < maxSlugLength; i++ {
		slug += "a"
	}
	if err := m.ValidateSlug(slug); err != nil {
		t.Errorf("ValidateSlug at max length unexpected error: %v", err)
	}

	longSlug := slug + "b"
	if err := m.ValidateSlug(longSlug); err == nil {
		t.Error("ValidateSlug should reject slug exceeding max length")
	}
}

func TestFindGitRoot(t *testing.T) {
	m := NewManager()
	dir := t.TempDir()
	t.Cleanup(func() { removeAllWithRetry(dir) })
	initGitRepo(t, dir)

	root, err := m.FindGitRoot(dir)
	if err != nil {
		t.Fatalf("FindGitRoot unexpected error: %v", err)
	}
	if root == "" {
		t.Error("FindGitRoot returned empty root")
	}
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		t.Errorf("FindGitRoot returned non-directory: %s", root)
	}
}

func TestFindGitRootNotRepo(t *testing.T) {
	m := NewManager()
	dir := t.TempDir()
	_, err := m.FindGitRoot(dir)
	if err == nil {
		t.Error("FindGitRoot should fail for non-git directory")
	}
}

func TestCreateAndRemoveWorktree(t *testing.T) {
	m := NewManager()
	dir := t.TempDir()
	t.Cleanup(func() { removeAllWithRetry(dir) })
	initGitRepo(t, dir)
	addCommit(t, dir)

	result, err := m.CreateWorktree(dir, "test-wt")
	if err != nil {
		t.Fatalf("CreateWorktree unexpected error: %v", err)
	}
	if result.Path == "" {
		t.Error("expected non-empty worktree path")
	}
	if result.Branch == "" {
		t.Error("expected non-empty branch name")
	}
	if info, err := os.Stat(result.Path); err != nil || !info.IsDir() {
		t.Errorf("worktree path does not exist: %s", result.Path)
	}

	if err := m.RemoveWorktree(result.Path, true); err != nil {
		t.Fatalf("RemoveWorktree unexpected error: %v", err)
	}
	if _, err := os.Stat(result.Path); !os.IsNotExist(err) {
		t.Error("worktree path should not exist after removal")
	}
}

func TestCreateWorktreeReuse(t *testing.T) {
	m := NewManager()
	dir := t.TempDir()
	t.Cleanup(func() { removeAllWithRetry(dir) })
	initGitRepo(t, dir)
	addCommit(t, dir)

	result1, err := m.CreateWorktree(dir, "test-reuse")
	if err != nil {
		t.Fatalf("first CreateWorktree error: %v", err)
	}
	result2, err := m.CreateWorktree(dir, "test-reuse")
	if err != nil {
		t.Fatalf("second CreateWorktree error: %v", err)
	}
	if result1.Path != result2.Path {
		t.Errorf("expected same path for reuse: %s vs %s", result1.Path, result2.Path)
	}

	m.RemoveWorktree(result1.Path, true)
}

func TestCountWorktreeChanges(t *testing.T) {
	m := NewManager()
	dir := t.TempDir()
	t.Cleanup(func() { removeAllWithRetry(dir) })
	initGitRepo(t, dir)
	addCommit(t, dir)

	result, err := m.CreateWorktree(dir, "test-changes")
	if err != nil {
		t.Fatalf("CreateWorktree error: %v", err)
	}
	defer m.RemoveWorktree(result.Path, true)

	changes := m.CountWorktreeChanges(result.Path)
	if changes == nil {
		t.Fatal("CountWorktreeChanges returned nil for clean worktree")
	}
	if changes.ChangedFiles > 0 {
		t.Errorf("expected 0 changed files, got %d", changes.ChangedFiles)
	}

	newFile := filepath.Join(result.Path, "newfile.txt")
	if err := os.WriteFile(newFile, []byte("test"), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	changes = m.CountWorktreeChanges(result.Path)
	if changes == nil {
		t.Fatal("CountWorktreeChanges returned nil")
	}
	if changes.ChangedFiles != 1 {
		t.Errorf("expected 1 changed file, got %d", changes.ChangedFiles)
	}
}

func TestRemoveWorktreeNonExistent(t *testing.T) {
	m := NewManager()
	err := m.RemoveWorktree("/nonexistent/path", true)
	if err == nil {
		t.Error("RemoveWorktree should fail for non-existent path")
	}
}

// Helpers

func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@test.com")
	runGit(t, dir, "config", "user.name", "Test")
}

func addCommit(t *testing.T, dir string) {
	t.Helper()
	dummy := filepath.Join(dir, "dummy.txt")
	if err := os.WriteFile(dummy, []byte("dummy"), 0o644); err != nil {
		t.Fatalf("write dummy: %v", err)
	}
	runGit(t, dir, "add", "dummy.txt")
	runGit(t, dir, "commit", "-m", "initial")
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %s: %v", args, string(out), err)
	}
}

// removeAllWithRetry retries os.RemoveAll to work around macOS TempDir
// cleanup races where .git directories may fail with ENOTEMPTY.
func removeAllWithRetry(dir string) {
	for i := 0; i < 5; i++ {
		if err := os.RemoveAll(dir); err == nil {
			return
		}
		time.Sleep(time.Duration(i+1) * 10 * time.Millisecond)
	}
}
