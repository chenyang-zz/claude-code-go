package file_read

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFileFreshnessWatcherLifecycle(t *testing.T) {
	tracker := NewFreshnessTracker()
	w := NewFileFreshnessWatcher(tracker)

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.go")
	if err := os.WriteFile(tmpFile, []byte("hello"), 0644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	if w.IsWatching(tmpFile) {
		t.Fatal("new watcher should not be watching any files")
	}

	if err := w.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	w.AddFile(tmpFile)
	if !w.IsWatching(tmpFile) {
		t.Fatal("expected file to be watched after AddFile")
	}

	w.RemoveFile(tmpFile)
	if w.IsWatching(tmpFile) {
		t.Fatal("expected file to not be watched after RemoveFile")
	}

	if err := w.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestFileFreshnessWatcherExternalChange(t *testing.T) {
	tracker := NewFreshnessTracker()
	w := NewFileFreshnessWatcher(tracker)

	if err := w.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer w.Stop()

	// Create a temp file to watch.
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "watched.txt")
	if err := os.WriteFile(tmpFile, []byte("hello"), 0644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	// Record the initial read state.
	tracker.RecordRead(tmpFile, time.Now())
	w.AddFile(tmpFile)

	// Wait for the watcher to be ready.
	time.Sleep(100 * time.Millisecond)

	// Modify the file externally.
	if err := os.WriteFile(tmpFile, []byte("changed"), 0644); err != nil {
		t.Fatalf("failed to modify temp file: %v", err)
	}

	// Wait for the fsnotify event to propagate.
	time.Sleep(200 * time.Millisecond)

	state, ok := tracker.GetState(tmpFile)
	if !ok {
		t.Fatal("expected state to exist")
	}
	if !state.HasChangedExternally {
		t.Fatal("expected HasChangedExternally to be true after external modification")
	}
}

func TestFileFreshnessWatcherDoubleStart(t *testing.T) {
	tracker := NewFreshnessTracker()
	w := NewFileFreshnessWatcher(tracker)

	if err := w.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	// Second start should be a no-op.
	if err := w.Start(); err != nil {
		t.Fatalf("second Start failed: %v", err)
	}
	if err := w.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestFileFreshnessWatcherStopWithoutStart(t *testing.T) {
	tracker := NewFreshnessTracker()
	w := NewFileFreshnessWatcher(tracker)

	if err := w.Stop(); err != nil {
		t.Fatalf("Stop without Start failed: %v", err)
	}
}

func TestFileFreshnessWatcherAddFileBeforeStart(t *testing.T) {
	tracker := NewFreshnessTracker()
	w := NewFileFreshnessWatcher(tracker)

	// Add file before starting - should record but not watch yet.
	w.AddFile("/tmp/test.go")
	if !w.IsWatching("/tmp/test.go") {
		t.Fatal("expected file to be recorded even before Start")
	}

	// Start should pick up the recorded file.
	if err := w.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer w.Stop()
}
