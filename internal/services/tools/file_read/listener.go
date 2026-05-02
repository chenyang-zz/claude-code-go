package file_read

import (
	"sync"
	"sync/atomic"
)

// ReadListener is called after a successful file read (text only).
// filePath is the absolute path, content is the full text content.
type ReadListener func(filePath string, content string)

type listenerEntry struct {
	id       uint64
	listener ReadListener
}

var (
	nextListenerID atomic.Uint64
	readListenersMu sync.Mutex
	readListeners   []listenerEntry
)

// RegisterReadListener registers a listener to be called after each successful text file read.
// Returns an unsubscribe function to remove the listener.
func RegisterReadListener(listener ReadListener) func() {
	if listener == nil {
		return func() {}
	}

	id := nextListenerID.Add(1)

	readListenersMu.Lock()
	readListeners = append(readListeners, listenerEntry{id: id, listener: listener})
	readListenersMu.Unlock()

	return func() {
		readListenersMu.Lock()
		defer readListenersMu.Unlock()
		for i, e := range readListeners {
			if e.id == id {
				// Remove by swapping with last element and truncating (order doesn't matter).
				readListeners[i] = readListeners[len(readListeners)-1]
				readListeners = readListeners[:len(readListeners)-1]
				return
			}
		}
	}
}

// ClearReadListeners removes all registered read listeners.
func ClearReadListeners() {
	readListenersMu.Lock()
	defer readListenersMu.Unlock()
	readListeners = nil
}

// notifyReadListeners calls all registered listeners with the given file path and content.
// Listeners are called without holding the lock to avoid deadlocks.
func notifyReadListeners(filePath string, content string) {
	readListenersMu.Lock()
	// Snapshot the current listeners under lock.
	snapshot := make([]ReadListener, len(readListeners))
	for i, e := range readListeners {
		snapshot[i] = e.listener
	}
	readListenersMu.Unlock()

	for _, l := range snapshot {
		l(filePath, content)
	}
}
