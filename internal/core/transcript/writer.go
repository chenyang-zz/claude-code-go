package transcript

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// Writer appends JSONL transcript entries to a file.
// It buffers writes internally and must be closed when no longer needed.
type Writer struct {
	mu       sync.Mutex
	file     *os.File
	buf      *bufio.Writer
	path     string
	entries  int64
	closed   bool
}

// NewWriter opens or creates a transcript file at the given path.
// Parent directories are created automatically with mode 0o700.
// The file itself is created with mode 0o600.
func NewWriter(path string) (*Writer, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("failed to create transcript directory: %w", err)
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, fmt.Errorf("failed to open transcript file: %w", err)
	}

	logger.DebugCF("transcript", "writer opened", map[string]any{
		"path": path,
	})

	return &Writer{
		file: file,
		buf:  bufio.NewWriter(file),
		path: path,
	}, nil
}

// WriteEntry serializes the given value as JSON and appends it as a line
// to the transcript file. The write is buffered; call Flush to ensure
// the entry is persisted to disk.
func (w *Writer) WriteEntry(entry any) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return fmt.Errorf("transcript writer is closed")
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal transcript entry: %w", err)
	}

	if _, err := w.buf.Write(data); err != nil {
		return fmt.Errorf("failed to write transcript entry: %w", err)
	}
	if err := w.buf.WriteByte('\n'); err != nil {
		return fmt.Errorf("failed to write transcript newline: %w", err)
	}

	w.entries++
	return nil
}

// Flush ensures all buffered entries are written to the underlying file.
func (w *Writer) Flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return fmt.Errorf("transcript writer is closed")
	}

	if err := w.buf.Flush(); err != nil {
		return fmt.Errorf("failed to flush transcript buffer: %w", err)
	}

	logger.DebugCF("transcript", "writer flushed", map[string]any{
		"path":    w.path,
		"entries": w.entries,
	})
	return nil
}

// Close flushes any remaining buffered data and closes the file.
func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return nil
	}
	w.closed = true

	var flushErr error
	if w.buf != nil {
		flushErr = w.buf.Flush()
	}

	var closeErr error
	if w.file != nil {
		closeErr = w.file.Close()
	}

	logger.DebugCF("transcript", "writer closed", map[string]any{
		"path":    w.path,
		"entries": w.entries,
	})

	if flushErr != nil {
		return fmt.Errorf("failed to flush on close: %w", flushErr)
	}
	if closeErr != nil {
		return fmt.Errorf("failed to close transcript file: %w", closeErr)
	}
	return nil
}

// Path returns the file path this writer is writing to.
func (w *Writer) Path() string {
	return w.path
}

// Entries returns the number of entries written so far.
func (w *Writer) Entries() int64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.entries
}
