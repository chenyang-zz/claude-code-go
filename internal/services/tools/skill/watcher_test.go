package skill

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewWatcher(t *testing.T) {
	w := NewWatcher()
	if w == nil {
		t.Fatal("expected non-nil watcher")
	}
	if w.started {
		t.Error("watcher should not be started initially")
	}
}

func TestWatcher_NilDirs(t *testing.T) {
	w := NewWatcher()
	defer w.Stop()

	if err := w.Start(nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should not crash with nil dirs.
}

func TestWatcher_EmptyDirs(t *testing.T) {
	w := NewWatcher()
	defer w.Stop()

	if err := w.Start([]string{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWatcher_NonExistentDirs(t *testing.T) {
	w := NewWatcher()
	defer w.Stop()

	dir := filepath.Join(t.TempDir(), "nonexistent")
	if err := w.Start([]string{dir}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should silently skip nonexistent dirs.
}

func TestWatcher_DoubleStart(t *testing.T) {
	w := NewWatcher()
	defer w.Stop()

	dir := t.TempDir()
	if err := w.Start([]string{dir}); err != nil {
		t.Fatalf("unexpected error on first start: %v", err)
	}
	if err := w.Start([]string{dir}); err != nil {
		t.Fatalf("unexpected error on second start: %v", err)
	}
}

func TestWatcher_StopBeforeStart(t *testing.T) {
	w := NewWatcher()
	if err := w.Stop(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWatcher_DoubleStop(t *testing.T) {
	w := NewWatcher()
	dir := t.TempDir()
	w.Start([]string{dir})
	w.Stop()
	if err := w.Stop(); err != nil {
		t.Fatalf("unexpected error on second stop: %v", err)
	}
}

func TestWatcher_FileChangeDetected(t *testing.T) {
	w := NewWatcher()
	defer w.Stop()

	dir := t.TempDir()
	if err := w.Start([]string{dir}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	received := make(chan struct{}, 1)
	unsub := OnDynamicSkillsLoaded(func() {
		select {
		case received <- struct{}{}:
		default:
		}
	})
	defer unsub()

	// Write a file to trigger a change event.
	path := filepath.Join(dir, "test.md")
	if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	// Wait for debounced callback.
	select {
	case <-received:
		// OK, callback fired.
	case <-time.After(2 * time.Second):
		t.Error("timed out waiting for reload callback")
	}
}

func TestWatcher_DebounceMultipleChanges(t *testing.T) {
	w := NewWatcher()
	defer w.Stop()

	dir := t.TempDir()
	if err := w.Start([]string{dir}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	count := 0
	done := make(chan struct{})
	unsub := OnDynamicSkillsLoaded(func() {
		count++
		close(done)
	})
	defer unsub()

	// Write multiple files rapidly.
	for i := 0; i < 5; i++ {
		path := filepath.Join(dir, "test"+string(rune('0'+i))+".md")
		os.WriteFile(path, []byte("test"), 0644)
		time.Sleep(10 * time.Millisecond)
	}

	select {
	case <-done:
		if count != 1 {
			t.Errorf("expected exactly 1 reload callback, got %d", count)
		}
	case <-time.After(2 * time.Second):
		t.Error("timed out waiting for reload callback")
	}
}

func TestIsGitPath(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{".git", true},
		{".git/config", true},
		{"project/.git/objects", true},
		{"project/subdir/.git/HEAD", true},
		{"project/node_modules/.git", true},
		{"src/main.go", false},
		{".claude/skills/my-skill/SKILL.md", false},
		{"skills/.gitignore", false}, // .gitignore is not inside .git/
	}
	for _, tc := range tests {
		if got := isGitPath(tc.path); got != tc.expected {
			t.Errorf("isGitPath(%q) = %v, want %v", tc.path, got, tc.expected)
		}
	}
}
