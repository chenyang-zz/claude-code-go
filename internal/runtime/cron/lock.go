package cron

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// lockFileRel is the path to the scheduler lock file relative to the project root.
const lockFileRel = ".claude/scheduled_tasks.lock"

// SchedulerLock is the content written to the lock file.
type SchedulerLock struct {
	SessionID  string `json:"sessionId"`
	PID        int    `json:"pid"`
	AcquiredAt int64  `json:"acquiredAt"`
}

var (
	lockMu           sync.Mutex
	lastBlockedBy    string
	unregisterCleanup func()
)

// lockFilePath returns the absolute path to the scheduler lock file.
func lockFilePath(projectRoot string) string {
	return filepath.Join(projectRoot, lockFileRel)
}

// TryAcquireSchedulerLock attempts to acquire the per-project scheduler lock
// for the given session. Only the owning session runs the check() loop; other
// sessions probe periodically to take over if the owner dies.
//
// Uses O_EXCL for atomic test-and-set. If the file exists:
//   - Already ours → true (idempotent re-acquire)
//   - Another live PID → false
//   - Stale (PID dead / corrupt) → unlink and retry exclusive create once
func TryAcquireSchedulerLock(projectRoot, sessionID string) (bool, error) {
	lockMu.Lock()
	defer lockMu.Unlock()

	lock := SchedulerLock{
		SessionID:  sessionID,
		PID:        os.Getpid(),
		AcquiredAt: nowMillis(),
	}

	if created, err := tryCreateExclusive(projectRoot, lock); err != nil {
		return false, err
	} else if created {
		lastBlockedBy = ""
		return true, nil
	}

	existing, err := readLockFile(projectRoot)
	if err != nil {
		return false, err
	}

	// Already ours (idempotent). After --resume the session ID is restored
	// but the process has a new PID — update the lock file so other sessions
	// see a live PID.
	if existing != nil && existing.SessionID == sessionID {
		if existing.PID != os.Getpid() {
			if err := writeLockFile(projectRoot, lock); err != nil {
				return false, err
			}
		}
		return true, nil
	}

	// Another live session — blocked.
	if existing != nil && isProcessRunning(existing.PID) {
		if lastBlockedBy != existing.SessionID {
			lastBlockedBy = existing.SessionID
		}
		return false, nil
	}

	// Stale — unlink and retry the exclusive create once.
	_ = os.Remove(lockFilePath(projectRoot))
	created, err := tryCreateExclusive(projectRoot, lock)
	if err != nil {
		return false, err
	}
	if created {
		lastBlockedBy = ""
		return true, nil
	}
	// Another session won the recovery race.
	return false, nil
}

// ReleaseSchedulerLock releases the scheduler lock if the current session owns
// it. Safe to call even if we don't own the lock.
func ReleaseSchedulerLock(projectRoot, sessionID string) error {
	lockMu.Lock()
	defer lockMu.Unlock()

	lastBlockedBy = ""

	existing, err := readLockFile(projectRoot)
	if err != nil || existing == nil {
		return nil
	}
	if existing.SessionID != sessionID {
		return nil
	}
	return os.Remove(lockFilePath(projectRoot))
}

// tryCreateExclusive creates the lock file atomically using O_EXCL.
func tryCreateExclusive(projectRoot string, lock SchedulerLock) (bool, error) {
	path := lockFilePath(projectRoot)
	data, err := json.Marshal(lock)
	if err != nil {
		return false, fmt.Errorf("marshal lock: %w", err)
	}

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return false, nil
		}
		// .claude/ might not exist yet — create it and retry once.
		if os.IsNotExist(err) {
			claudeDir := filepath.Join(projectRoot, ".claude")
			if mkErr := os.MkdirAll(claudeDir, 0o755); mkErr != nil {
				return false, mkErr
			}
			f2, err2 := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
			if err2 != nil {
				if os.IsExist(err2) {
					return false, nil
				}
				return false, err2
			}
			defer f2.Close()
			_, err2 = f2.Write(data)
			return err2 == nil, err2
		}
		return false, err
	}
	defer f.Close()
	_, err = f.Write(data)
	return err == nil, err
}

// readLockFile reads and parses the lock file. Returns nil if the file does not
// exist or is malformed.
func readLockFile(projectRoot string) (*SchedulerLock, error) {
	raw, err := os.ReadFile(lockFilePath(projectRoot))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var lock SchedulerLock
	if err := json.Unmarshal(raw, &lock); err != nil {
		return nil, nil // malformed → treat as non-existent
	}
	return &lock, nil
}

// writeLockFile overwrites the lock file with fresh content.
func writeLockFile(projectRoot string, lock SchedulerLock) error {
	data, err := json.Marshal(lock)
	if err != nil {
		return fmt.Errorf("marshal lock: %w", err)
	}
	return os.WriteFile(lockFilePath(projectRoot), data, 0o644)
}

// isProcessRunning checks whether a process with the given PID is running by
// sending signal 0 (null signal) to it.
func isProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// nowMillis returns the current time in epoch milliseconds.
func nowMillis() int64 {
	return time.Now().UnixMilli()
}

// GetPIDString returns the PID as a string for logging purposes.
func GetPIDString() string {
	return strconv.Itoa(os.Getpid())
}

// findProcessByPID parses a PID from a string. Exported for testing.
func findProcessByPID(pidStr string) (int, error) {
	return strconv.Atoi(strings.TrimSpace(pidStr))
}
