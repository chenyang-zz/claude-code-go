package plugin

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewWatcher(t *testing.T) {
	var called bool
	w := NewWatcher(func() { called = true })
	if w == nil {
		t.Fatal("NewWatcher returned nil")
	}
	if w.onChange == nil {
		t.Error("onChange should be set")
	}
	if w.started {
		t.Error("new watcher should not be started")
	}
	_ = called
}

func TestWatcher_StartStop(t *testing.T) {
	dir := t.TempDir()
	w := NewWatcher(func() {})

	if err := w.Start([]string{dir}); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	if !w.started {
		t.Error("watcher should be started")
	}

	if err := w.Stop(); err != nil {
		t.Fatalf("Stop error: %v", err)
	}
	if w.started {
		t.Error("watcher should be stopped")
	}
}

func TestWatcher_StartSkipsMissingDirs(t *testing.T) {
	w := NewWatcher(func() {})
	if err := w.Start([]string{"/nonexistent/path/12345"}); err != nil {
		t.Fatalf("Start should not error for missing dirs: %v", err)
	}
	if !w.started {
		t.Error("watcher should be marked started even with no valid dirs")
	}
	w.Stop()
}

func TestWatcher_StartRecursive(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub1", "sub2")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	w := NewWatcher(func() {})
	if err := w.Start([]string{dir}); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	defer w.Stop()

	if w.fsw == nil {
		t.Fatal("fsnotify watcher should be set")
	}
}

func TestWatcher_Debounce(t *testing.T) {
	dir := t.TempDir()

	var count atomic.Int32
	w := NewWatcher(func() {
		count.Add(1)
	})

	if err := w.Start([]string{dir}); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	defer w.Stop()

	// Write multiple files rapidly — should trigger only one callback.
	for i := 0; i < 5; i++ {
		f := filepath.Join(dir, "test.txt")
		_ = os.WriteFile(f, []byte("hello"), 0o644)
		_ = os.Remove(f)
	}

	// Wait for debounce + a bit of margin.
	time.Sleep(450 * time.Millisecond)

	if count.Load() != 1 {
		t.Fatalf("expected 1 callback after debounce, got %d", count.Load())
	}
}

func TestWatcher_DebounceResets(t *testing.T) {
	dir := t.TempDir()

	var count atomic.Int32
	w := NewWatcher(func() {
		count.Add(1)
	})

	if err := w.Start([]string{dir}); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	defer w.Stop()

	// First write.
	_ = os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644)
	time.Sleep(150 * time.Millisecond)

	// Second write within debounce window — should reset timer.
	_ = os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b"), 0o644)

	// Wait long enough for first timer to have fired if not reset.
	time.Sleep(200 * time.Millisecond)

	// Only one callback should fire after the second write's timer expires.
	time.Sleep(200 * time.Millisecond)

	if count.Load() != 1 {
		t.Fatalf("expected 1 callback after reset debounce, got %d", count.Load())
	}
}

func TestWatcher_StopRecreatesStopCh(t *testing.T) {
	dir := t.TempDir()
	w := NewWatcher(func() {})

	if err := w.Start([]string{dir}); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	if err := w.Stop(); err != nil {
		t.Fatalf("Stop error: %v", err)
	}

	// Restart should work because stopCh was recreated.
	if err := w.Start([]string{dir}); err != nil {
		t.Fatalf("Restart error: %v", err)
	}
	if !w.started {
		t.Error("watcher should be restarted")
	}
	w.Stop()
}

func TestIsGitPath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/project/.git/config", true},
		{"/project/.git", true},
		{"/project/.gitignore", false},
		{"/project/src/main.go", false},
		{"/.git/config", true},
	}
	for _, tc := range tests {
		if got := isGitPath(tc.path); got != tc.want {
			t.Errorf("isGitPath(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestIsTemporaryFile(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/project/file.go~", true},
		{"/project/file.tmp", true},
		{"/project/file.swp", true},
		{"/project/file.swo", true},
		{"/project/.DS_Store", true},
		{"/project/file.go", false},
		{"/project/plugin.json", false},
		{"/project/README.md", false},
	}
	for _, tc := range tests {
		if got := isTemporaryFile(tc.path); got != tc.want {
			t.Errorf("isTemporaryFile(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}
