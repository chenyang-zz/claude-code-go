package sessionmemory

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const (
	// sessionMemoryDirName is the directory name under ~/.claude/.
	sessionMemoryDirName = "session-memory"
	// sessionMemoryFileName is the file name for the session memory summary.
	sessionMemoryFileName = "summary.md"
)

// DefaultSessionMemoryTemplate is the default content written into a newly
// created session memory summary file.
// prompt.go references this constant.
const DefaultSessionMemoryTemplate = `# Session Title
_A short and distinctive 5-10 word descriptive title for the session. Super info dense, no filler_

# Current State
_What is actively being worked on right now? Pending tasks not yet completed. Immediate next steps._

# Task specification
_What did the user ask to build? Any design decisions or other explanatory context_

# Files and Functions
_What are the important files? In short, what do they contain and why are they relevant?_

# Workflow
_What bash commands are usually run and in what order? How to interpret their output if not obvious?_

# Errors & Corrections
_Errors encountered and how they were fixed. What did the user correct? What approaches failed and should not be tried again?_

# Codebase and System Documentation
_What are the important system components? How do they work/fit together?_

# Learnings
_What has worked well? What has not? What to avoid? Do not duplicate items from other sections_

# Key results
_If the user asked a specific output such as an answer to a question, a table, or other document, repeat the exact result here_

# Worklog
_Step by step, what was attempted, done? Very terse summary for each step_
`

// GetSessionMemoryDir returns the session memory directory path
// (~/.claude/session-memory/).
func GetSessionMemoryDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		logger.WarnF("failed to get user home dir, falling back to relative path", map[string]any{"error": err.Error()})
		return filepath.Join(".claude", sessionMemoryDirName) + string(filepath.Separator)
	}
	return filepath.Join(home, ".claude", sessionMemoryDirName) + string(filepath.Separator)
}

// GetSessionMemoryPath returns the session memory summary file path
// (~/.claude/session-memory/summary.md).
func GetSessionMemoryPath() string {
	return filepath.Join(GetSessionMemoryDir(), sessionMemoryFileName)
}

// SetupSessionMemoryFile ensures the session memory directory and file exist.
// If the file is newly created, it is pre-populated with DefaultSessionMemoryTemplate.
// Returns the file path, its current content, and any error encountered.
func SetupSessionMemoryFile(ctx context.Context) (memoryPath string, currentMemory string, err error) {
	memoryPath = GetSessionMemoryPath()

	// Ensure the session memory directory exists.
	dir := GetSessionMemoryDir()
	if mkdirErr := os.MkdirAll(dir, 0700); mkdirErr != nil {
		err = mkdirErr
		return
	}
	logger.Infof("session memory directory ensured: %s", dir)

	// Attempt to create the file exclusively. If it already exists we skip
	// writing the template; if created fresh we write the default content.
	f, openErr := os.OpenFile(memoryPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if openErr == nil {
		// File did not exist — write the default template.
		if _, writeErr := f.WriteString(DefaultSessionMemoryTemplate); writeErr != nil {
			f.Close()
			err = writeErr
			return
		}
		f.Close()
		logger.Infof("session memory file created with default template: %s", memoryPath)
	} else if !errors.Is(openErr, os.ErrExist) {
		// An error other than "file already exists" — propagate it.
		err = openErr
		return
	}

	// Read the current content of the file.
	content, readErr := os.ReadFile(memoryPath)
	if readErr != nil {
		err = readErr
		return
	}
	currentMemory = string(content)

	logger.Infof("session memory file loaded, length=%d", len(currentMemory))

	return
}

// GetSessionMemoryContent reads and returns the content of the session memory
// summary file. If the file does not exist it returns an empty string instead
// of an error.
func GetSessionMemoryContent(ctx context.Context) (string, error) {
	memoryPath := GetSessionMemoryPath()

	content, err := os.ReadFile(memoryPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			logger.WarnF("session memory file not found", map[string]any{"path": memoryPath})
			return "", nil
		}
		return "", err
	}

	logger.Infof("session memory content loaded, length=%d", len(content))

	return string(content), nil
}
