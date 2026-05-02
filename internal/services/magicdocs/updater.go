package magicdocs

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// FileReader abstracts file reading for Magic Docs updates.
// The implementation uses FileReadTool or direct filesystem access.
type FileReader interface {
	// ReadFile reads a text file and returns its content.
	ReadFile(filePath string) (content string, err error)
}

// Updater handles the Magic Docs update pipeline.
type Updater struct {
	runner SubagentRunner
	reader FileReader
}

// NewUpdater creates a Magic Docs updater with the given dependencies.
func NewUpdater(runner SubagentRunner, reader FileReader) *Updater {
	return &Updater{runner: runner, reader: reader}
}

// UpdateAllDocs is the post-turn hook handler. It iterates all tracked Magic Docs
// and updates each one by launching a forked subagent.
func (u *Updater) UpdateAllDocs(ctx context.Context, messages []message.Message) error {
	if u == nil || u.runner == nil {
		return nil
	}
	if !HasTrackedDocs() {
		return nil
	}

	docs := TrackedDocs()
	for _, doc := range docs {
		if doc == nil || doc.Path == "" {
			continue
		}
		if err := u.updateOneDoc(ctx, doc.Path, messages); err != nil {
			logger.WarnCF("magicdocs", "failed to update magic doc", map[string]any{
				"file_path": doc.Path,
				"error":     err.Error(),
			})
			// Continue with other docs even if one fails.
		}
	}
	return nil
}

// updateOneDoc runs the full update pipeline for a single Magic Doc.
func (u *Updater) updateOneDoc(ctx context.Context, filePath string, messages []message.Message) error {
	// 1. Re-read the document to get the latest content.
	currentContent, err := u.readFile(filePath)
	if err != nil {
		// File deleted or unreadable — remove from tracking.
		UnregisterMagicDoc(filePath)
		logger.DebugCF("magicdocs", "magic doc unregistered (unreadable)", map[string]any{
			"file_path": filePath,
			"error":     err.Error(),
		})
		return nil
	}

	// 2. Re-detect the Magic Doc header from latest content.
	info := DetectMagicDocHeader(currentContent)
	if info == nil {
		// Header removed — stop tracking.
		UnregisterMagicDoc(filePath)
		logger.DebugCF("magicdocs", "magic doc header removed, unregistered", map[string]any{
			"file_path": filePath,
		})
		return nil
	}

	// 3. Build the update prompt.
	updatePrompt := BuildUpdatePrompt(currentContent, filePath, info.Title, info.Instructions)

	// 4. Launch the forked subagent to update the document.
	logger.DebugCF("magicdocs", "launching magic doc update subagent", map[string]any{
		"file_path": filePath,
		"title":     info.Title,
	})
	return u.runner.RunSubagent(ctx, filePath, updatePrompt, messages)
}

// readFile reads a file's text content using the configured reader.
func (u *Updater) readFile(filePath string) (string, error) {
	if u.reader != nil {
		return u.reader.ReadFile(filePath)
	}
	// Fallback to direct filesystem read.
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("read magic doc %s: %w", filePath, err)
	}
	return string(data), nil
}

// directFileReader is a simple FileReader that uses os.ReadFile.
type directFileReader struct{}

func (r *directFileReader) ReadFile(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("read file %s: %w", filePath, err)
	}
	return string(data), nil
}

// HasToolCallsInLastTurn checks if the latest assistant message has pending tool calls.
// This mirrors the TS hasToolCallsInLastAssistantTurn function.
func HasToolCallsInLastTurn(messages []message.Message) bool {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == message.RoleAssistant {
			return hasToolCallContent(messages[i].Content)
		}
	}
	return false
}

// hasToolCallContent checks if any content block is a tool_use.
func hasToolCallContent(parts []message.ContentPart) bool {
	for _, p := range parts {
		if p.Type == "tool_use" {
			return true
		}
	}
	return false
}

// IsMainThread checks messages for the repl_main_thread query source marker.
// The post-turn hook should only fire for main thread conversations.
// Since querySource is not directly available in the Go message model,
// we approximate by checking the first user message content.
func IsMainThread(messages []message.Message) bool {
	for _, m := range messages {
		if m.Role == message.RoleUser && len(m.Content) > 0 {
			// Skip assistant/synthetic messages. Main thread has real user messages.
			for _, p := range m.Content {
				if p.Type == "text" && strings.TrimSpace(p.Text) != "" {
					return true
				}
			}
		}
	}
	return false
}
