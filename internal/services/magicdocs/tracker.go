package magicdocs

import "sync"

// TrackedDoc represents a tracked Magic Doc file.
type TrackedDoc struct {
	Path string
}

var (
	trackedDocsMu sync.RWMutex
	trackedDocs   = make(map[string]*TrackedDoc)
)

// RegisterMagicDoc adds a file path to the tracked Magic Docs set (idempotent).
func RegisterMagicDoc(filePath string) {
	trackedDocsMu.Lock()
	defer trackedDocsMu.Unlock()
	if _, exists := trackedDocs[filePath]; !exists {
		trackedDocs[filePath] = &TrackedDoc{Path: filePath}
	}
}

// UnregisterMagicDoc removes a file path from tracking.
func UnregisterMagicDoc(filePath string) {
	trackedDocsMu.Lock()
	defer trackedDocsMu.Unlock()
	delete(trackedDocs, filePath)
}

// ClearTrackedMagicDocs removes all tracked Magic Docs.
func ClearTrackedMagicDocs() {
	trackedDocsMu.Lock()
	defer trackedDocsMu.Unlock()
	clear(trackedDocs)
}

// TrackedDocs returns a snapshot of all tracked Magic Docs.
func TrackedDocs() []*TrackedDoc {
	trackedDocsMu.RLock()
	defer trackedDocsMu.RUnlock()
	docs := make([]*TrackedDoc, 0, len(trackedDocs))
	for _, d := range trackedDocs {
		docs = append(docs, d)
	}
	return docs
}

// HasTrackedDocs returns true if there are any tracked Magic Docs.
func HasTrackedDocs() bool {
	trackedDocsMu.RLock()
	defer trackedDocsMu.RUnlock()
	return len(trackedDocs) > 0
}
