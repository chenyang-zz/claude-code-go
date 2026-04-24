package transcript

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// Reader reads JSONL transcript entries from a file.
// It supports sequential read, error recovery (skipping malformed lines),
// and type-discriminated deserialization into the concrete entry types
// defined in entry.go.
type Reader struct {
	scanner *bufio.Scanner
	file    *os.File
	path    string
	line    int
}

// NewReader opens a transcript file for sequential reading.
// The caller must call Close when done.
func NewReader(path string) (*Reader, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open transcript file: %w", err)
	}

	logger.DebugCF("transcript", "reader opened", map[string]any{
		"path": path,
	})

	return &Reader{
		scanner: bufio.NewScanner(file),
		file:    file,
		path:    path,
	}, nil
}

// ReadNext reads and returns the next entry from the transcript.
// It returns io.EOF when all entries have been consumed.
// Malformed lines are logged, counted as skipped, and reading continues
// with the next line so that a single corrupt entry does not abort the
// whole recovery.
func (r *Reader) ReadNext() (any, error) {
	for r.scanner.Scan() {
		r.line++
		line := r.scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		entry, err := r.parseLine(line)
		if err != nil {
			logger.WarnCF("transcript", "skipping malformed transcript line", map[string]any{
				"path": r.path,
				"line": r.line,
				"error": err.Error(),
			})
			continue
		}

		return entry, nil
	}

	if err := r.scanner.Err(); err != nil {
		return nil, fmt.Errorf("transcript read error at line %d: %w", r.line, err)
	}

	return nil, io.EOF
}

// ReadAll reads every remaining entry from the transcript and returns
// them in file order. Malformed lines are skipped with a warning.
func (r *Reader) ReadAll() ([]any, error) {
	var entries []any
	for {
		entry, err := r.ReadNext()
		if err == io.EOF {
			break
		}
		if err != nil {
			return entries, err
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// Close closes the underlying file.
func (r *Reader) Close() error {
	if r.file == nil {
		return nil
	}

	logger.DebugCF("transcript", "reader closed", map[string]any{
		"path": r.path,
		"lines_read": r.line,
	})

	return r.file.Close()
}

// Path returns the file path this reader is reading from.
func (r *Reader) Path() string {
	return r.path
}

// parseLine unmarshals one JSONL line into the matching concrete entry type.
func (r *Reader) parseLine(line []byte) (any, error) {
	var disc typeDiscriminator
	if err := json.Unmarshal(line, &disc); err != nil {
		return nil, fmt.Errorf("failed to parse type discriminator: %w", err)
	}

	switch disc.Type {
	case "user":
		var entry UserEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			return nil, fmt.Errorf("failed to unmarshal user entry: %w", err)
		}
		return entry, nil
	case "assistant":
		var entry AssistantEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			return nil, fmt.Errorf("failed to unmarshal assistant entry: %w", err)
		}
		return entry, nil
	case "tool_use":
		var entry ToolUseEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			return nil, fmt.Errorf("failed to unmarshal tool_use entry: %w", err)
		}
		return entry, nil
	case "tool_result":
		var entry ToolResultEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			return nil, fmt.Errorf("failed to unmarshal tool_result entry: %w", err)
		}
		return entry, nil
	case "system":
		var entry SystemEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			return nil, fmt.Errorf("failed to unmarshal system entry: %w", err)
		}
		return entry, nil
	case "summary":
		var entry SummaryEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			return nil, fmt.Errorf("failed to unmarshal summary entry: %w", err)
		}
		return entry, nil
	default:
		return nil, fmt.Errorf("unknown transcript entry type %q", disc.Type)
	}
}

// typeDiscriminator is the minimal shape needed to read the "type" field
// before dispatching to the concrete entry struct.
type typeDiscriminator struct {
	Type string `json:"type"`
}
