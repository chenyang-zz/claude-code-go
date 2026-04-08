package git

import (
	"context"
	"testing"
)

// TestParseWorktreePathsKeepsCurrentFirst verifies porcelain parsing keeps the active worktree ahead of sorted siblings.
func TestParseWorktreePathsKeepsCurrentFirst(t *testing.T) {
	output := "worktree /repo/main\nHEAD abc\nbranch refs/heads/main\n\nworktree /repo/wt-b\nHEAD def\nbranch refs/heads/wt-b\n\nworktree /repo/wt-a\nHEAD ghi\nbranch refs/heads/wt-a\n"

	got := parseWorktreePaths(output, "/repo/main/subdir")

	want := []string{"/repo/main", "/repo/wt-a", "/repo/wt-b"}
	if len(got) != len(want) {
		t.Fatalf("parseWorktreePaths() len = %d, want %d (%#v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("parseWorktreePaths()[%d] = %q, want %q; all = %#v", i, got[i], want[i], got)
		}
	}
}

// TestParseWorktreePathsReturnsSortedOthers verifies parsing still returns a stable sorted list when cwd is outside every worktree.
func TestParseWorktreePathsReturnsSortedOthers(t *testing.T) {
	output := "worktree /repo/wt-b\n\nworktree /repo/wt-a\n"

	got := parseWorktreePaths(output, "/elsewhere")

	want := []string{"/repo/wt-a", "/repo/wt-b"}
	if len(got) != len(want) {
		t.Fatalf("parseWorktreePaths() len = %d, want %d (%#v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("parseWorktreePaths()[%d] = %q, want %q; all = %#v", i, got[i], want[i], got)
		}
	}
}

// TestClientListWorktreesFallsBackWhenGitMissing verifies Git discovery failures do not bubble up into the REPL path.
func TestClientListWorktreesFallsBackWhenGitMissing(t *testing.T) {
	client := Client{Command: "/definitely/missing/git"}

	got, err := client.ListWorktrees(context.Background(), "/repo")
	if err != nil {
		t.Fatalf("ListWorktrees() error = %v, want nil", err)
	}
	if len(got) != 0 {
		t.Fatalf("ListWorktrees() = %#v, want empty fallback", got)
	}
}
