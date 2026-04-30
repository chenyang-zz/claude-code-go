package plugin

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// defaultDebounceInterval is the time window for merging rapid file change
// events into a single reload notification. 300ms matches the skill watcher
// and the TS RELOAD_DEBOUNCE_MS.
const defaultDebounceInterval = 300 * time.Millisecond

// Watcher watches plugin directories for file changes and notifies the
// registered callback when the plugin set may have changed.
type Watcher struct {
	fsw           *fsnotify.Watcher
	onChange      func()
	debounceTimer *time.Timer
	debounceMu    sync.Mutex
	started       bool
	running       bool
	stopCh        chan struct{}
	mu            sync.Mutex
}

// NewWatcher creates a new plugin directory watcher. The onChange callback is
// invoked (with debouncing) whenever a relevant file change is detected in any
// watched directory.
func NewWatcher(onChange func()) *Watcher {
	return &Watcher{
		onChange: onChange,
		stopCh:   make(chan struct{}),
	}
}

// Start creates the underlying fsnotify watcher and begins monitoring the
// given directories and all their subdirectories for file changes.  Directories
// that do not exist are silently skipped.
func (w *Watcher) Start(dirs []string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.started {
		return nil
	}

	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	w.fsw = fsw

	added := 0
	for _, dir := range dirs {
		dir = filepath.Clean(dir)
		if dir == "" {
			continue
		}
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			continue
		}

		// Recursively watch the directory and all subdirectories so that
		// deep plugin structures (commands/, agents/, etc.) are covered.
		if err := w.addDirRecursive(fsw, dir); err != nil {
			logger.DebugCF("plugin.watcher", "failed to watch directory tree", map[string]any{
				"dir":   dir,
				"error": err.Error(),
			})
			continue
		}
		added++
	}

	if added == 0 {
		_ = fsw.Close()
		w.fsw = nil
		w.started = true
		return nil
	}

	logger.DebugCF("plugin.watcher", "started watching directories", map[string]any{
		"count": added,
	})

	w.started = true
	w.running = true
	go w.loop(fsw)

	return nil
}

// addDirRecursive adds dir and every subdirectory beneath it to the fsnotify
// watcher.
func (w *Watcher) addDirRecursive(fsw *fsnotify.Watcher, dir string) error {
	if err := fsw.Add(dir); err != nil {
		return err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil // non-fatal: we at least added the parent
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		sub := filepath.Join(dir, entry.Name())
		if err := w.addDirRecursive(fsw, sub); err != nil {
			logger.DebugCF("plugin.watcher", "failed to watch subdirectory", map[string]any{
				"dir":   sub,
				"error": err.Error(),
			})
		}
	}
	return nil
}

// Stop gracefully shuts down the file watcher.  It closes the underlying
// fsnotify watcher, stops the debounce timer, and recreates the stop channel
// so the watcher can be restarted later.
func (w *Watcher) Stop() error {
	w.mu.Lock()
	if !w.started {
		w.mu.Unlock()
		return nil
	}
	w.started = false
	w.running = false
	close(w.stopCh)

	if w.fsw != nil {
		_ = w.fsw.Close()
		w.fsw = nil
	}
	w.mu.Unlock()

	w.debounceMu.Lock()
	if w.debounceTimer != nil {
		w.debounceTimer.Stop()
		w.debounceTimer = nil
	}
	w.debounceMu.Unlock()

	// Recreate stopCh so Start() can be called again.
	w.mu.Lock()
	w.stopCh = make(chan struct{})
	w.mu.Unlock()

	logger.DebugCF("plugin.watcher", "stopped", nil)
	return nil
}

// loop is the main event loop that consumes fsnotify events and triggers
// debounced reloads.
func (w *Watcher) loop(fsw *fsnotify.Watcher) {
	for {
		select {
		case <-w.stopCh:
			return
		case event, ok := <-fsw.Events:
			if !ok {
				return
			}
			w.handleEvent(event)
		case err, ok := <-fsw.Errors:
			if !ok {
				return
			}
			logger.DebugCF("plugin.watcher", "fsnotify error", map[string]any{
				"error": err.Error(),
			})
		}
	}
}

// handleEvent processes a single fsnotify event.  It filters out irrelevant
// operations, .git directories, and temporary files before scheduling a
// debounced reload.
func (w *Watcher) handleEvent(event fsnotify.Event) {
	// Filter out events that don't indicate content changes.
	if event.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Remove|fsnotify.Rename) == 0 {
		return
	}

	// Ignore .git directories.
	if isGitPath(event.Name) {
		return
	}

	// Ignore temporary and editor swap files.
	if isTemporaryFile(event.Name) {
		return
	}

	logger.DebugCF("plugin.watcher", "file change detected", map[string]any{
		"path": event.Name,
		"op":   event.Op.String(),
	})

	w.scheduleReload()
}

// scheduleReload triggers a debounced reload.  If a reload is already pending,
// the timer is reset, effectively merging rapid changes into a single callback.
func (w *Watcher) scheduleReload() {
	w.debounceMu.Lock()
	defer w.debounceMu.Unlock()

	if w.debounceTimer != nil {
		w.debounceTimer.Stop()
	}

	w.debounceTimer = time.AfterFunc(defaultDebounceInterval, func() {
		w.debounceMu.Lock()
		w.debounceTimer = nil
		w.debounceMu.Unlock()

		if w.onChange != nil {
			logger.DebugCF("plugin.watcher", "firing plugin reload", nil)
			w.onChange()
		}
	})
}

// isGitPath reports whether the given path contains a .git component.
func isGitPath(path string) bool {
	return strings.Contains(filepath.ToSlash(path), "/.git/") ||
		strings.HasSuffix(filepath.ToSlash(path), "/.git")
}

// isTemporaryFile reports whether the given path is a temporary or editor
// swap file that should be ignored.
func isTemporaryFile(path string) bool {
	base := filepath.Base(path)
	return strings.HasSuffix(base, "~") ||
		strings.HasSuffix(base, ".tmp") ||
		strings.HasSuffix(base, ".swp") ||
		strings.HasSuffix(base, ".swo") ||
		base == ".DS_Store"
}
