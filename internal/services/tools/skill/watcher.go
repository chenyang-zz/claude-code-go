package skill

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// DefaultWatcher is the process-level skill file watcher. It is started during
// bootstrap after skill loading and runs until process exit.
var DefaultWatcher = NewWatcher()

// StartDefaultWatcher resolves the standard skill and command directories from
// homeDir and projectPath and starts watching them for changes.
func StartDefaultWatcher(homeDir, projectPath string) {
	var dirs []string

	if homeDir != "" {
		userSkillsDir := filepath.Join(homeDir, ".claude", "skills")
		dirs = append(dirs, userSkillsDir)
		userCommandsDir := filepath.Join(homeDir, ".claude", "commands")
		dirs = append(dirs, userCommandsDir)
	}

	if projectPath != "" {
		projectSkillsDir := filepath.Join(projectPath, ".claude", "skills")
		dirs = append(dirs, projectSkillsDir)
		projectCommandsDir := filepath.Join(projectPath, ".claude", "commands")
		dirs = append(dirs, projectCommandsDir)
	}

	if err := DefaultWatcher.Start(dirs); err != nil {
		logger.WarnCF("skill.watcher", "failed to start default watcher", map[string]any{
			"error": err.Error(),
		})
	}
}

// defaultDebounceInterval is the time window for merging rapid file change events
// into a single reload notification. 300ms matches the TS RELOAD_DEBOUNCE_MS.
const defaultDebounceInterval = 300 * time.Millisecond

// Watcher watches skill and command directories for file changes and notifies
// registered listeners when the skill set may have changed.
type Watcher struct {
	fsw             *fsnotify.Watcher
	debounceTimer   *time.Timer
	debounceMu      sync.Mutex
	debouncePending bool
	started         bool
	running         bool
	stopCh          chan struct{}
	mu              sync.Mutex
}

// NewWatcher creates a new Watcher that can be used to monitor skill and command
// directories for file changes.
func NewWatcher() *Watcher {
	return &Watcher{
		stopCh: make(chan struct{}),
	}
}

// Start creates the underlying fsnotify watcher and begins monitoring the given
// directories for file changes. Directories that do not exist are silently
// skipped. When a file change event is detected, the watcher debounces for the
// configured interval and then fires all OnDynamicSkillsLoaded callbacks.
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
		// Walk one level deep to catch SKILL.md and commands/*.md files,
		// and add the directory itself.
		if err := fsw.Add(dir); err != nil {
			logger.DebugCF("skill.watcher", "failed to watch directory", map[string]any{
				"dir":   dir,
				"error": err.Error(),
			})
			continue
		}
		added++

		// Walk one level deep to pick up subdirectories (skill dirs, command subdirs).
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			sub := filepath.Join(dir, entry.Name())
			if err := fsw.Add(sub); err != nil {
				continue
			}
		}
	}

	if added == 0 {
		fsw.Close()
		w.fsw = nil
		w.started = true
		return nil
	}

	logger.DebugCF("skill.watcher", "started watching directories", map[string]any{
		"count": added,
	})

	w.started = true
	w.running = true
	go w.loop(fsw)

	return nil
}

// Stop gracefully shuts down the file watcher. It closes the underlying
// fsnotify watcher and waits for the event loop to exit.
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
		w.fsw.Close()
		w.fsw = nil
	}
	w.mu.Unlock()

	w.debounceMu.Lock()
	if w.debounceTimer != nil {
		w.debounceTimer.Stop()
		w.debounceTimer = nil
	}
	w.debouncePending = false
	w.debounceMu.Unlock()

	logger.DebugCF("skill.watcher", "stopped", nil)
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
			logger.DebugCF("skill.watcher", "fsnotify error", map[string]any{
				"error": err.Error(),
			})
		}
	}
}

// handleEvent processes a single fsnotify event. It filters out .git directories
// and non-relevant file types, then triggers a debounced reload.
func (w *Watcher) handleEvent(event fsnotify.Event) {
	// Filter out events that don't indicate content changes.
	if event.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Remove|fsnotify.Rename) == 0 {
		return
	}

	// Ignore .git directories.
	if isGitPath(event.Name) {
		return
	}

	logger.DebugCF("skill.watcher", "file change detected", map[string]any{
		"path": event.Name,
		"op":   event.Op.String(),
	})

	w.scheduleReload()
}

// scheduleReload triggers a debounced reload. If a reload is already pending,
// the timer is reset, effectively merging rapid changes into a single reload.
func (w *Watcher) scheduleReload() {
	w.debounceMu.Lock()
	defer w.debounceMu.Unlock()

	if w.debounceTimer != nil {
		w.debounceTimer.Stop()
	}

	w.debounceTimer = time.AfterFunc(defaultDebounceInterval, func() {
		w.debounceMu.Lock()
		w.debouncePending = false
		w.debounceTimer = nil
		w.debounceMu.Unlock()

		logger.DebugCF("skill.watcher", "firing skills reload", nil)
		emitSkillsLoaded()
	})
	w.debouncePending = true
}

// isGitPath reports whether the given path contains a .git component, indicating
// it resides inside a git directory that should be ignored.
func isGitPath(path string) bool {
	return slices.Contains(strings.Split(filepath.ToSlash(path), "/"), ".git")
}
