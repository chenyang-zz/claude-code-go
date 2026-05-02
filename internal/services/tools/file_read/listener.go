package file_read

import "sync"

// ReadListener is called after a successful file read (text only).
// filePath is the absolute path, content is the full text content.
type ReadListener func(filePath string, content string)

var (
	readListenersMu sync.Mutex
	readListeners   []ReadListener
)

// RegisterReadListener registers a listener to be called after each successful text file read.
// Returns an unsubscribe function to remove the listener.
func RegisterReadListener(listener ReadListener) func() {
	if listener == nil {
		return func() {}
	}

	readListenersMu.Lock()
	defer readListenersMu.Unlock()

	readListeners = append(readListeners, listener)
	idx := len(readListeners) - 1
	removed := false

	return func() {
		readListenersMu.Lock()
		defer readListenersMu.Unlock()
		if !removed {
			readListeners[idx] = nil
			removed = true
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
	// Compact nil entries while copying the slice.
	listeners := make([]ReadListener, 0, len(readListeners))
	for _, l := range readListeners {
		if l != nil {
			listeners = append(listeners, l)
		}
	}
	readListeners = listeners
	readListenersMu.Unlock()

	for _, l := range listeners {
		l(filePath, content)
	}
}
