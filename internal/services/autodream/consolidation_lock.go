package autodream

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/services/extractmemories"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const (
	// lockFileName is the name of the consolidation lock file.
	lockFileName = ".consolidate-lock"
	// holderStaleMs is the duration after which a lock holder is considered
	// stale, even if the PID is still alive (PID reuse guard).
	holderStaleMs = 60 * 60 * 1000 // 1 hour
)

// lockPath returns the absolute path to the consolidation lock file
// for the given project root.
func lockPath(projectRoot string) string {
	return filepath.Join(extractmemories.GetAutoMemPath(projectRoot), lockFileName)
}

// readLastConsolidatedAt returns the mtime of the lock file in milliseconds
// since Unix epoch. Returns 0 if the file does not exist.
func readLastConsolidatedAt(projectRoot string) (int64, error) {
	info, err := os.Stat(lockPath(projectRoot))
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	return info.ModTime().UnixMilli(), nil
}

// tryAcquireConsolidationLock attempts to acquire the consolidation lock.
// Returns the mtime (in ms) before acquisition on success, or -1 if the lock
// is held by another live process with a fresh lock.
func tryAcquireConsolidationLock(projectRoot string) (int64, error) {
	path := lockPath(projectRoot)

	// Read existing lock info.
	var priorMtime int64
	var holderPid int
	info, err := os.Stat(path)
	if err == nil {
		priorMtime = info.ModTime().UnixMilli()
		if time.Now().UnixMilli()-priorMtime < holderStaleMs {
			data, readErr := os.ReadFile(path)
			if readErr == nil {
				trimmed := strings.TrimSpace(string(data))
				if pid, parseErr := strconv.Atoi(trimmed); parseErr == nil {
					holderPid = pid
				}
			}
			if holderPid > 0 && isProcessRunning(holderPid) {
				logger.DebugCF("autodream", "lock held by live PID", map[string]any{
					"pid":      holderPid,
					"age_secs": (time.Now().UnixMilli() - priorMtime) / 1000,
				})
				return -1, nil
			}
			// Dead PID or unparseable body — reclaim.
		}
	} else if os.IsNotExist(err) {
		priorMtime = 0
	} else {
		return -1, err
	}

	// Ensure memory dir exists.
	memDir := extractmemories.GetAutoMemPath(projectRoot)
	if mkdirErr := os.MkdirAll(memDir, 0755); mkdirErr != nil {
		return -1, mkdirErr
	}

	// Write our PID.
	if writeErr := os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())), 0644); writeErr != nil {
		return -1, writeErr
	}

	// Race verification: re-read to confirm we are the writer.
	data, readErr := os.ReadFile(path)
	if readErr != nil {
		return -1, readErr
	}
	trimmed := strings.TrimSpace(string(data))
	if parsed, parseErr := strconv.Atoi(trimmed); parseErr != nil || parsed != os.Getpid() {
		return -1, nil // Lost the race.
	}

	return priorMtime, nil
}

// rollbackConsolidationLock rewinds the lock mtime after a failed fork.
// priorMtime of 0 means the lock did not exist before — unlink it.
// Otherwise, clear the body and restore the prior mtime.
func rollbackConsolidationLock(projectRoot string, priorMtime int64) {
	path := lockPath(projectRoot)
	if priorMtime == 0 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			logger.DebugCF("autodream", "rollback unlink failed", map[string]any{
				"error": err.Error(),
			})
		}
		return
	}
	// Clear the PID body first so WriteFile doesn't update mtime after we restore it.
	if err := os.WriteFile(path, []byte{}, 0644); err != nil {
		logger.DebugCF("autodream", "rollback body clear failed", map[string]any{
			"error": err.Error(),
		})
	}
	t := time.UnixMilli(priorMtime)
	if err := os.Chtimes(path, t, t); err != nil {
		logger.DebugCF("autodream", "rollback chtimes failed — next trigger delayed to minHours", map[string]any{
			"error": err.Error(),
		})
	}
}

// recordConsolidation stamps the lock file with the current PID and mtime.
// Optimistic — fires at prompt-build time, best-effort.
func recordConsolidation(projectRoot string) {
	memDir := extractmemories.GetAutoMemPath(projectRoot)
	if err := os.MkdirAll(memDir, 0755); err != nil {
		logger.DebugCF("autodream", "recordConsolidation mkdir failed", map[string]any{
			"error": err.Error(),
		})
		return
	}
	if err := os.WriteFile(lockPath(projectRoot), []byte(strconv.Itoa(os.Getpid())), 0644); err != nil {
		logger.DebugCF("autodream", "recordConsolidation write failed", map[string]any{
			"error": err.Error(),
		})
	}
}

// listSessionsTouchedSince scans the transcript directory for session files
// with mtime after sinceMs (in milliseconds since Unix epoch).
// Returns session IDs (filenames without .jsonl extension).
func listSessionsTouchedSince(projectRoot string, sinceMs int64) ([]string, error) {
	transcriptDir := filepath.Join(projectRoot, ".claude", "transcripts")
	entries, err := os.ReadDir(transcriptDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var sessionIds []string
	sinceTime := time.UnixMilli(sinceMs)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".jsonl") {
			continue
		}
		// Skip agent session files.
		if strings.HasPrefix(name, "agent-") {
			continue
		}
		info, statErr := entry.Info()
		if statErr != nil {
			continue
		}
		if info.ModTime().After(sinceTime) {
			sessionIds = append(sessionIds, strings.TrimSuffix(name, ".jsonl"))
		}
	}

	return sessionIds, nil
}

// getSessionID returns the current session ID from the environment.
func getSessionID() string {
	return os.Getenv("CLAUDE_CODE_SESSION_ID")
}

// isProcessRunning checks whether a process with the given PID is running
// by sending signal 0 (null signal).
func isProcessRunning(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || err == syscall.EPERM
}
