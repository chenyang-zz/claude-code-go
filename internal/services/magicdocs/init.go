package magicdocs

import (
	"context"
	"os"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
	file_read "github.com/sheepzhao/claude-code-go/internal/services/tools/file_read"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// PostTurnHookFunc is the signature for post-turn hook registration callbacks.
// It matches engine.PostTurnHook.
type PostTurnHookFunc func(ctx context.Context, messages []message.Message, workingDir string) error

// InitMagicDocs initializes the Magic Docs system by registering:
//  1. A FileReadListener that detects "# MAGIC DOC:" headers when files are read.
//  2. A PostTurnHook that triggers document updates after each complete turn.
//
// The runner is a SubagentRunner that handles the actual subagent execution.
// Pass nil to skip subagent execution (only detection and tracking will work).
// The registerPostTurnHook function registers a hook with the engine's post-turn system.
func InitMagicDocs(runner SubagentRunner, registerPostTurnHook func(hook PostTurnHookFunc)) {
	if runner == nil {
		logger.DebugCF("magicdocs", "skipping init: no subagent runner available", nil)
		return
	}
	if !isMagicDocsEnabled() {
		logger.DebugCF("magicdocs", "skipping init: magic docs disabled", nil)
		return
	}

	// Create the updater with a direct filesystem reader.
	updater := NewUpdater(runner, &directFileReader{})

	// 1. Register FileReadListener — detect Magic Doc headers when files are read.
	file_read.RegisterReadListener(func(filePath string, content string) {
		// Only track markdown files.
		if !isMarkdownFile(filePath) {
			return
		}
		info := DetectMagicDocHeader(content)
		if info != nil {
			RegisterMagicDoc(filePath)
			logger.DebugCF("magicdocs", "magic doc detected", map[string]any{
				"file_path": filePath,
				"title":     info.Title,
			})
		}
	})

	// 2. Register PostTurnHook — update tracked docs after each complete turn.
	registerPostTurnHook(PostTurnHookFunc(func(ctx context.Context, messages []message.Message, workingDir string) error {
		// Only fire for main thread conversations (skip subagent turns).
		if !IsMainThread(messages) {
			return nil
		}
		// Skip if there are pending tool calls.
		if HasToolCallsInLastTurn(messages) {
			return nil
		}
		return updater.UpdateAllDocs(ctx, messages)
	}))

	logger.DebugCF("magicdocs", "magic docs initialized", nil)
}

// isMagicDocsEnabled checks whether Magic Docs should be enabled.
// Currently checks the CLAUDE_CODE_MAGIC_DOCS env var; defaults to enabled.
func isMagicDocsEnabled() bool {
	v := strings.ToLower(os.Getenv("CLAUDE_CODE_MAGIC_DOCS"))
	if v == "0" || v == "false" || v == "no" {
		return false
	}
	// Default enabled when env var is unset or truthy.
	return true
}

// isMarkdownFile checks if a file path has a markdown extension.
func isMarkdownFile(filePath string) bool {
	lower := strings.ToLower(filePath)
	return strings.HasSuffix(lower, ".md") || strings.HasSuffix(lower, ".markdown")
}
