package autodream

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestLockPath(t *testing.T) {
	projectRoot := "/tmp/test-project"
	path := lockPath(projectRoot)
	if path == "" {
		t.Error("expected non-empty lock path")
	}
	if filepath.Base(path) != lockFileName {
		t.Errorf("expected lock file name %s, got %s", lockFileName, filepath.Base(path))
	}
}

func TestReadLastConsolidatedAt_NoFile(t *testing.T) {
	dir := t.TempDir()
	// Set memory base dir to temp dir for isolation.
	os.Setenv("CLAUDE_CODE_REMOTE_MEMORY_DIR", filepath.Join(dir, "memory-base"))
	defer os.Unsetenv("CLAUDE_CODE_REMOTE_MEMORY_DIR")

	mtime, err := readLastConsolidatedAt(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mtime != 0 {
		t.Errorf("expected mtime=0 when no lock file, got %d", mtime)
	}
}

func TestTryAcquireConsolidationLock_FirstAcquire(t *testing.T) {
	dir := t.TempDir()
	memDir := filepath.Join(dir, "memory-base")
	os.Setenv("CLAUDE_CODE_REMOTE_MEMORY_DIR", memDir)
	defer os.Unsetenv("CLAUDE_CODE_REMOTE_MEMORY_DIR")

	priorMtime, err := tryAcquireConsolidationLock(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if priorMtime < 0 {
		t.Error("expected successful lock acquisition")
	}
	if priorMtime != 0 {
		t.Errorf("expected priorMtime=0 for first acquire, got %d", priorMtime)
	}

	// Verify lock file exists.
	path := lockPath(dir)
	if _, err := os.Stat(path); err != nil {
		t.Errorf("lock file should exist after acquire: %v", err)
	}
}

func TestTryAcquireConsolidationLock_SecondAcquireBlocked(t *testing.T) {
	dir := t.TempDir()
	memDir := filepath.Join(dir, "memory-base")
	os.Setenv("CLAUDE_CODE_REMOTE_MEMORY_DIR", memDir)
	defer os.Unsetenv("CLAUDE_CODE_REMOTE_MEMORY_DIR")

	// First acquire.
	_, err := tryAcquireConsolidationLock(dir)
	if err != nil {
		t.Fatalf("first acquire failed: %v", err)
	}

	// Second acquire should be blocked (lock is fresh and holder PID is our own,
	// which is running).
	priorMtime, err := tryAcquireConsolidationLock(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if priorMtime != -1 {
		t.Errorf("expected lock block (priorMtime=-1), got %d", priorMtime)
	}
}

func TestRollbackConsolidationLock_Unlink(t *testing.T) {
	dir := t.TempDir()
	memDir := filepath.Join(dir, "memory-base")
	os.Setenv("CLAUDE_CODE_REMOTE_MEMORY_DIR", memDir)
	defer os.Unsetenv("CLAUDE_CODE_REMOTE_MEMORY_DIR")

	_, err := tryAcquireConsolidationLock(dir)
	if err != nil {
		t.Fatalf("acquire failed: %v", err)
	}

	rollbackConsolidationLock(dir, 0)

	// Lock file should be removed.
	path := lockPath(dir)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("lock file should be unlinked after rollback with priorMtime=0")
	}
}

func TestRollbackConsolidationLock_RestoreMtime(t *testing.T) {
	dir := t.TempDir()
	memDir := filepath.Join(dir, "memory-base")
	os.Setenv("CLAUDE_CODE_REMOTE_MEMORY_DIR", memDir)
	defer os.Unsetenv("CLAUDE_CODE_REMOTE_MEMORY_DIR")

	_, err := tryAcquireConsolidationLock(dir)
	if err != nil {
		t.Fatalf("acquire failed: %v", err)
	}

	path := lockPath(dir)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat failed: %v", err)
	}
	currentMtime := info.ModTime()

	// Rollback to a much earlier time.
	oldTime := time.Now().Add(-48 * time.Hour)
	rollbackConsolidationLock(dir, oldTime.UnixMilli())

	info2, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat after rollback failed: %v", err)
	}
	if info2.ModTime().Equal(currentMtime) {
		t.Error("expected mtime to be restored after rollback")
	}
}

func TestRecordConsolidation(t *testing.T) {
	dir := t.TempDir()
	memDir := filepath.Join(dir, "memory-base")
	os.Setenv("CLAUDE_CODE_REMOTE_MEMORY_DIR", memDir)
	defer os.Unsetenv("CLAUDE_CODE_REMOTE_MEMORY_DIR")

	recordConsolidation(dir)

	path := lockPath(dir)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("lock file should exist: %v", err)
	}
	pid, err := strconv.Atoi(string(data))
	if err != nil {
		t.Fatalf("lock body should be a PID: %v", err)
	}
	if pid != os.Getpid() {
		t.Errorf("expected PID %d, got %d", os.Getpid(), pid)
	}
}

func TestListSessionsTouchedSince_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	sessionIds, err := listSessionsTouchedSince(dir, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessionIds) != 0 {
		t.Errorf("expected empty list, got %d sessions", len(sessionIds))
	}
}

func TestIsProcessRunning_Self(t *testing.T) {
	if !isProcessRunning(os.Getpid()) {
		t.Error("expected current process to be running")
	}
}

func TestIsProcessRunning_InvalidPID(t *testing.T) {
	if isProcessRunning(99999999) {
		t.Error("expected invalid PID to not be running")
	}
}
