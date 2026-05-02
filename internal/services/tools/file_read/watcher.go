package file_read

import (
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// FileFreshnessWatcher watches individual files for external changes and
// notifies a FreshnessTracker when a watched file is modified.
type FileFreshnessWatcher struct {
	fsw      *fsnotify.Watcher
	tracker  *FreshnessTracker
	started  bool
	stopCh   chan struct{}
	mu       sync.Mutex
	watched  map[string]struct{}
	watchMu  sync.RWMutex
}

// NewFileFreshnessWatcher creates a new watcher bound to the given tracker.
func NewFileFreshnessWatcher(tracker *FreshnessTracker) *FileFreshnessWatcher {
	return &FileFreshnessWatcher{
		tracker: tracker,
		stopCh:  make(chan struct{}),
		watched: make(map[string]struct{}),
	}
}

// Start initializes the underlying fsnotify watcher and begins the event loop.
func (w *FileFreshnessWatcher) Start() error {
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
	w.started = true
	go w.loop(fsw)
	return nil
}

// Stop shuts down the watcher and releases all resources.
func (w *FileFreshnessWatcher) Stop() error {
	w.mu.Lock()
	if !w.started {
		w.mu.Unlock()
		return nil
	}
	w.started = false
	close(w.stopCh)
	if w.fsw != nil {
		w.fsw.Close()
		w.fsw = nil
	}
	w.mu.Unlock()

	w.watchMu.Lock()
	w.watched = make(map[string]struct{})
	w.watchMu.Unlock()
	return nil
}

// AddFile registers a file path for change monitoring. If the watcher is not
// started, the path is recorded and will be watched when Start is called.
func (w *FileFreshnessWatcher) AddFile(filePath string) {
	if filePath == "" {
		return
	}

	w.watchMu.Lock()
	defer w.watchMu.Unlock()
	if _, ok := w.watched[filePath]; ok {
		return
	}

	w.watched[filePath] = struct{}{}

	w.mu.Lock()
	fsw := w.fsw
	w.mu.Unlock()
	if fsw != nil {
		if err := fsw.Add(filePath); err != nil {
			logger.DebugCF("file_read.watcher", "failed to watch file", map[string]any{
				"path":  filePath,
				"error": err.Error(),
			})
			delete(w.watched, filePath)
		}
	}
}

// RemoveFile unregisters a file path from change monitoring.
func (w *FileFreshnessWatcher) RemoveFile(filePath string) {
	if filePath == "" {
		return
	}

	w.watchMu.Lock()
	defer w.watchMu.Unlock()
	if _, ok := w.watched[filePath]; !ok {
		return
	}

	delete(w.watched, filePath)

	w.mu.Lock()
	fsw := w.fsw
	w.mu.Unlock()
	if fsw != nil {
		_ = fsw.Remove(filePath)
	}
}

// IsWatching reports whether the given file path is currently being watched.
func (w *FileFreshnessWatcher) IsWatching(filePath string) bool {
	w.watchMu.RLock()
	defer w.watchMu.RUnlock()
	_, ok := w.watched[filePath]
	return ok
}

// loop consumes fsnotify events and updates the freshness tracker.
func (w *FileFreshnessWatcher) loop(fsw *fsnotify.Watcher) {
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
			logger.DebugCF("file_read.watcher", "fsnotify error", map[string]any{
				"error": err.Error(),
			})
		}
	}
}

// handleEvent processes a single fsnotify event.
func (w *FileFreshnessWatcher) handleEvent(event fsnotify.Event) {
	// Only react to content-modifying operations.
	if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename) == 0 {
		return
	}

	w.watchMu.RLock()
	_, isWatched := w.watched[event.Name]
	w.watchMu.RUnlock()
	if !isWatched {
		return
	}

	logger.DebugCF("file_read.watcher", "file changed externally", map[string]any{
		"path": event.Name,
		"op":   event.Op.String(),
	})

	w.tracker.MarkChanged(event.Name)
}
