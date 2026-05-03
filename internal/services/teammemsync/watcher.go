package teammemsync

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const debounceDuration = 2 * time.Second

// WatcherConfig holds dependencies for the team memory file watcher.
type WatcherConfig struct {
	// TeamDir is the absolute path to the team memory directory to watch.
	TeamDir string
	// PullFunc performs an initial pull on startup.
	PullFunc func(ctx context.Context) (*FetchResult, error)
	// PushFunc pushes local changes to the server.
	PushFunc func(ctx context.Context) (*PushResult, error)
}

// TeamMemoryWatcher watches the team memory directory for file changes and
// triggers debounced push operations. It performs an initial pull on startup,
// then uses fsnotify to detect Create, Write, and Remove events. In-flight
// pushes block Stop so that pending changes are flushed on shutdown.
type TeamMemoryWatcher struct {
	teamDir  string
	pullFunc func(ctx context.Context) (*FetchResult, error)
	pushFunc func(ctx context.Context) (*PushResult, error)

	mu       sync.Mutex
	pushCond *sync.Cond

	watcher              *fsnotify.Watcher
	debounceTimer        *time.Timer
	pushInProgress       bool
	hasPendingChanges    bool
	pushSuppressedReason string
}

// NewTeamMemoryWatcher creates a new watcher instance with the supplied
// configuration. The caller must call Start before the watcher is active.
func NewTeamMemoryWatcher(cfg WatcherConfig) *TeamMemoryWatcher {
	w := &TeamMemoryWatcher{
		teamDir:  cfg.TeamDir,
		pullFunc: cfg.PullFunc,
		pushFunc: cfg.PushFunc,
	}
	w.pushCond = sync.NewCond(&w.mu)
	return w
}

// Start begins watching the team memory directory. It creates the directory if
// needed, runs the initial pull (before the watcher starts so pulled files
// don't trigger a spurious push), then starts an fsnotify watcher on the
// entire directory tree. Errors from directory creation, initial pull, or
// watcher setup are logged but not returned — NotifyWrite remains functional
// even when the filesystem watcher is unavailable.
func (w *TeamMemoryWatcher) Start(ctx context.Context) error {
	// Create the team memory directory if needed (idempotent).
	if err := os.MkdirAll(w.teamDir, 0755); err != nil {
		logger.WarnCF("teammemsync", "failed to create team memory dir", map[string]any{
			"dir":   w.teamDir,
			"error": err.Error(),
		})
	}

	// Initial pull from server runs before the watcher starts. Its disk writes
	// won't trigger schedulePush since the watcher isn't active yet.
	if w.pullFunc != nil {
		result, err := w.pullFunc(ctx)
		if err != nil {
			logger.WarnCF("teammemsync", "initial pull error", map[string]any{
				"error": err.Error(),
			})
		} else if result != nil && !result.Success && result.Error != "" {
			logger.WarnCF("teammemsync", "initial pull failed", map[string]any{
				"error": result.Error,
			})
		}
	}

	// Create fsnotify watcher.
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		logger.WarnCF("teammemsync", "failed to create fsnotify watcher", map[string]any{
			"error": err.Error(),
		})
		return nil
	}

	// Walk the team directory tree and add every directory to the watcher.
	if err := walkDirs(w.teamDir, fw); err != nil {
		fw.Close()
		logger.WarnCF("teammemsync", "failed to watch team memory dirs", map[string]any{
			"dir":   w.teamDir,
			"error": err.Error(),
		})
		return nil
	}

	w.mu.Lock()
	w.watcher = fw
	w.mu.Unlock()

	// Start the event loop in a background goroutine.
	go w.eventLoop(fw)

	logger.DebugCF("teammemsync", "watching team memory dir", map[string]any{
		"dir": w.teamDir,
	})
	return nil
}

// Stop gracefully shuts down the watcher. It stops the debounce timer, closes
// the filesystem watcher, waits for any in-flight push to complete, and
// flushes pending changes with a final push if one is not suppressed. The
// flush is best-effort — if the context deadline is exceeded the call is
// abandoned.
func (w *TeamMemoryWatcher) Stop(ctx context.Context) {
	w.mu.Lock()

	// Stop the debounce timer so no new pushes are scheduled.
	if w.debounceTimer != nil {
		w.debounceTimer.Stop()
		w.debounceTimer = nil
	}

	// Close the fsnotify watcher (signals the event loop to exit).
	var fw *fsnotify.Watcher
	if w.watcher != nil {
		fw = w.watcher
		w.watcher = nil
	}
	w.mu.Unlock()

	if fw != nil {
		fw.Close()
	}

	// Wait for any in-flight push to complete.
	w.mu.Lock()
	for w.pushInProgress {
		w.pushCond.Wait()
	}
	needsFlush := w.hasPendingChanges && w.pushSuppressedReason == ""
	w.mu.Unlock()

	// Flush pending changes that were debounced but not yet pushed.
	if needsFlush && w.pushFunc != nil {
		_, _ = w.pushFunc(ctx)
	}
}

// NotifyWrite schedules an explicit push, for example from PostToolUse hooks.
// This is the caller-driven path that works even when the filesystem watcher
// is unavailable or misses events.
func (w *TeamMemoryWatcher) NotifyWrite() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.schedulePushLocked()
}

// IsPermanentFailure reports whether a push result represents a failure that
// cannot self-heal on retry without user action. Returns true for no_oauth and
// no_repo error types, and for any 4xx HTTP status code except 409 Conflict
// and 429 Rate Limit. When a permanent failure is detected the watcher stops
// scheduling new pushes until a file deletion (Remove event) clears the
// suppression.
func IsPermanentFailure(r *PushResult) bool {
	if r.ErrorType == "no_oauth" || r.ErrorType == "no_repo" {
		return true
	}
	if r.HTTPStatus >= 400 && r.HTTPStatus < 500 &&
		r.HTTPStatus != 409 && r.HTTPStatus != 429 {
		return true
	}
	return false
}

// eventLoop reads fsnotify events and errors until the watcher is closed.
func (w *TeamMemoryWatcher) eventLoop(fw *fsnotify.Watcher) {
	for {
		select {
		case event, ok := <-fw.Events:
			if !ok {
				return
			}
			w.handleEvent(event)
		case err, ok := <-fw.Errors:
			if !ok {
				return
			}
			if err != nil {
				logger.WarnCF("teammemsync", "fsnotify error", map[string]any{
					"error": err.Error(),
				})
			}
		}
	}
}

// handleEvent processes a single fsnotify event. Remove events clear push
// suppression. Create, Write, and Remove events schedule a debounced push.
// When a Create event refers to a new directory, handleEvent walks the new
// directory tree and adds all subdirectories to the watcher (fsnotify does
// not support recursive watching natively).
func (w *TeamMemoryWatcher) handleEvent(event fsnotify.Event) {
	w.mu.Lock()

	// Remove events indicate file deletion — clear suppression.
	if event.Has(fsnotify.Remove) && w.pushSuppressedReason != "" {
		logger.InfoCF("teammemsync", "unlink cleared suppression", map[string]any{
			"was_reason": w.pushSuppressedReason,
		})
		w.pushSuppressedReason = ""
	}

	// Schedule a push for content-changing events.
	if event.Has(fsnotify.Create) || event.Has(fsnotify.Write) || event.Has(fsnotify.Remove) {
		w.schedulePushLocked()
	}

	isCreateDir := event.Has(fsnotify.Create)
	currentWatcher := w.watcher
	w.mu.Unlock()

	// When a new directory is created, fsnotify will not watch its contents
	// because it doesn't support recursive watching. Walk the new directory
	// and add all subdirectories to the watcher.
	if isCreateDir && currentWatcher != nil {
		if fi, err := os.Stat(event.Name); err == nil && fi.IsDir() {
			if walkErr := walkDirs(event.Name, currentWatcher); walkErr != nil {
				logger.WarnCF("teammemsync", "failed to add new dir to watcher", map[string]any{
					"dir":   event.Name,
					"error": walkErr.Error(),
				})
			}
		}
	}
}

// schedulePushLocked registers a debounced push. It resets the debounce timer
// so rapid successive writes are coalesced into a single push. Must be called
// with w.mu held.
func (w *TeamMemoryWatcher) schedulePushLocked() {
	if w.pushSuppressedReason != "" {
		return
	}
	w.hasPendingChanges = true
	if w.debounceTimer != nil {
		w.debounceTimer.Stop()
	}
	w.debounceTimer = time.AfterFunc(debounceDuration, w.executePush)
}

// executePush performs the actual push. If a push is already in progress it
// re-schedules so that the subsequent push runs after the current one
// completes.
func (w *TeamMemoryWatcher) executePush() {
	w.mu.Lock()
	if w.pushInProgress {
		// A push is already running — wait for it to finish and push again.
		w.schedulePushLocked()
		w.mu.Unlock()
		return
	}
	w.pushInProgress = true
	w.mu.Unlock()

	if w.pushFunc == nil {
		w.mu.Lock()
		w.pushInProgress = false
		w.pushCond.Broadcast()
		w.mu.Unlock()
		return
	}

	result, err := w.pushFunc(context.Background())

	w.mu.Lock()
	defer func() {
		w.pushInProgress = false
		w.pushCond.Broadcast()
		w.mu.Unlock()
	}()

	if err != nil {
		logger.WarnCF("teammemsync", "push error", map[string]any{
			"error": err.Error(),
		})
		return
	}

	if result.Success {
		w.hasPendingChanges = false
		if result.FilesUploaded > 0 {
			logger.InfoCF("teammemsync", "pushed files", map[string]any{
				"files_uploaded": result.FilesUploaded,
			})
		}
		return
	}

	// Push failed.
	logger.WarnCF("teammemsync", "push failed", map[string]any{
		"error":  result.Error,
		"status": result.HTTPStatus,
	})

	if IsPermanentFailure(result) && w.pushSuppressedReason == "" {
		if result.HTTPStatus > 0 {
			w.pushSuppressedReason = fmt.Sprintf("http_%d", result.HTTPStatus)
		} else {
			w.pushSuppressedReason = result.ErrorType
			if w.pushSuppressedReason == "" {
				w.pushSuppressedReason = "unknown"
			}
		}
		logger.WarnCF("teammemsync", "suppressing retry until next unlink or restart", map[string]any{
			"reason": w.pushSuppressedReason,
		})
	}
}

// walkDirs recursively adds all directories under root to the fsnotify
// watcher. Permission-denied and not-found errors are silently skipped.
func walkDirs(root string, fw *fsnotify.Watcher) error {
	return filepath.Walk(root, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			if os.IsPermission(err) || os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if fi.IsDir() {
			if addErr := fw.Add(path); addErr != nil {
				return addErr
			}
		}
		return nil
	})
}
