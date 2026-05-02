package extractmemories

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// SubagentRunner executes a forked subagent with the given messages.
// The messages contain the user prompt instructing the agent to update
// memory files.
type SubagentRunner interface {
	Run(ctx context.Context, messages []message.Message) error
}

// System orchestrates the extractMemories pipeline: cursor tracking,
// coalesce, forked subagent execution, and graceful drain.
type System struct {
	state       *State
	runner      SubagentRunner
	projectRoot string

	// inflight tracks in-progress extractions for drain.
	inflight sync.WaitGroup
}

// NewSystem creates a new extractMemories system with the given runner
// and project root. Pass nil runner to skip subagent execution (detection
// and prompt building still work).
func NewSystem(runner SubagentRunner, projectRoot string) *System {
	return &System{
		state:       NewState(),
		runner:      runner,
		projectRoot: projectRoot,
	}
}

// defaultToolNames maps Go tool display names to the canonical names used in
// tool_use blocks.
var (
	readToolNames  = []string{"Read", "FileReadTool"}
	writeToolNames = []string{"Write", "FileWriteTool"}
	editToolNames  = []string{"Edit", "FileEditTool"}
	grepToolNames  = []string{"Grep", "GrepTool"}
	globToolNames  = []string{"Glob", "GlobTool"}
	bashToolNames  = []string{"Bash", "BashTool"}
)

// toolNameMatches checks if a tool_use block's ToolName matches one of the
// given names.
func toolNameMatches(toolName string, names []string) bool {
	for _, n := range names {
		if strings.EqualFold(toolName, n) {
			return true
		}
	}
	return false
}

// isReadToolUse returns true if the block is a Read tool_use.
func isReadToolUse(toolName string) bool {
	return toolNameMatches(toolName, readToolNames)
}

// isWriteOrEditToolUse returns true if the block is a Write or Edit tool_use.
func isWriteOrEditToolUse(toolName string) bool {
	return toolNameMatches(toolName, writeToolNames) || toolNameMatches(toolName, editToolNames)
}

// getWrittenFilePath extracts the file_path from a tool_use content block,
// if present. Returns empty string when the block is not an Edit/Write tool
// use or has no file_path.
func getWrittenFilePath(block message.ContentPart) string {
	if block.Type != "tool_use" {
		return ""
	}
	if !isWriteOrEditToolUse(block.ToolName) {
		return ""
	}
	if fp, ok := block.ToolInput["file_path"]; ok {
		if s, ok := fp.(string); ok {
			return s
		}
	}
	return ""
}

// isModelVisibleMessage returns true for messages that are visible to the model
// (sent in API calls). Excludes progress, system, and attachment messages.
func isModelVisibleMessage(msg message.Message) bool {
	return msg.Role == message.RoleUser || msg.Role == message.RoleAssistant
}

// countModelVisibleMessagesSince counts model-visible messages after the given
// index. If lastIndex < 0, counts all model-visible messages. If messages have
// been compacted (total count less than lastIndex), falls back to full count.
func countModelVisibleMessagesSince(messages []message.Message, lastIndex int) int {
	if lastIndex < 0 || lastIndex >= len(messages) {
		// Cursor invalid (possibly compacted away). Count all.
		count := 0
		for _, msg := range messages {
			if isModelVisibleMessage(msg) {
				count++
			}
		}
		return count
	}

	count := 0
	for i := lastIndex + 1; i < len(messages); i++ {
		if isModelVisibleMessage(messages[i]) {
			count++
		}
	}
	return count
}

// hasMemoryWritesSince returns true if any assistant message after the cursor
// index contains a Write/Edit tool_use block targeting an auto-memory path.
func hasMemoryWritesSince(messages []message.Message, lastIndex int, projectRoot string) bool {
	startIdx := 0
	if lastIndex >= 0 && lastIndex < len(messages) {
		startIdx = lastIndex + 1
	}
	for i := startIdx; i < len(messages); i++ {
		msg := messages[i]
		if msg.Role != message.RoleAssistant {
			continue
		}
		for _, block := range msg.Content {
			filePath := getWrittenFilePath(block)
			if filePath != "" && IsAutoMemPath(filePath, projectRoot) {
				return true
			}
		}
	}
	return false
}

// extractWrittenPaths collects unique file paths written by the forked agent.
func extractWrittenPaths(messages []message.Message) []string {
	seen := make(map[string]bool)
	var paths []string
	for _, msg := range messages {
		if msg.Role != message.RoleAssistant {
			continue
		}
		for _, block := range msg.Content {
			fp := getWrittenFilePath(block)
			if fp != "" && !seen[fp] {
				seen[fp] = true
				paths = append(paths, fp)
			}
		}
	}
	return paths
}

// runExtraction is the core extraction loop. It counts new messages since the
// cursor, checks mutual exclusion, scans existing memory files, builds the
// extraction prompt, runs the forked subagent, and advances the cursor.
func (s *System) runExtraction(ctx context.Context, messages []message.Message, isTrailingRun bool) {
	lastIdx := s.state.GetLastMessageIndex()
	newMessageCount := countModelVisibleMessagesSince(messages, lastIdx)

	// Mutual exclusion: if the main agent already wrote to memory files,
	// skip the forked agent and advance the cursor.
	if hasMemoryWritesSince(messages, lastIdx, s.projectRoot) {
		logger.DebugCF("extractmemories", "skipping — conversation already wrote to memory files", nil)
		if len(messages) > 0 {
			s.state.SetLastMessageIndex(len(messages) - 1)
		}
		return
	}

	// Throttling: skip turns below the interval threshold (unless trailing).
	if !isTrailingRun {
		s.state.IncrementTurnsSinceLastExtraction()
		cfg := s.state.Config()
		if s.state.TurnsSinceLastExtraction() < cfg.ExtractIntervalTurns {
			return
		}
	}
	s.state.ResetTurnsSinceLastExtraction()

	logger.DebugCF("extractmemories", "starting extraction", map[string]any{
		"new_message_count": newMessageCount,
		"project_root":      s.projectRoot,
	})

	// Scan existing memory files and format a manifest so the forked agent
	// doesn't spend a turn on ls.
	memDir := GetAutoMemPath(s.projectRoot)
	existingHeaders, err := ScanMemoryFiles(memDir)
	if err != nil {
		logger.WarnCF("extractmemories", "failed to scan memory files", map[string]any{
			"error": err.Error(),
		})
	}
	existingManifest := FormatMemoryManifest(existingHeaders)

	// Build the extraction prompt.
	cfg := s.state.Config()
	prompt := BuildExtractAutoOnlyPrompt(newMessageCount, existingManifest, cfg.SkipIndex)

	// Create the user message for the forked subagent.
	promptMsg := message.Message{
		Role: message.RoleUser,
		Content: []message.ContentPart{
			message.TextPart(prompt),
		},
	}

	if s.runner != nil {
		if err := s.runner.Run(ctx, []message.Message{promptMsg}); err != nil {
			logger.WarnCF("extractmemories", "subagent execution failed", map[string]any{
				"error": err.Error(),
			})
		}
	} else {
		logger.DebugCF("extractmemories", "subagent runner is nil, skipping execution", nil)
	}

	// Advance the cursor after a successful run.
	if len(messages) > 0 {
		s.state.SetLastMessageIndex(len(messages) - 1)
	}

	logger.DebugCF("extractmemories", "extraction completed", nil)
}

// PostTurnHook returns a function matching the engine.PostTurnHook signature
// that can be registered directly via engine.RegisterPostTurnHook.
func (s *System) PostTurnHook() func(ctx context.Context, messages []message.Message, workingDir string) error {
	return func(ctx context.Context, messages []message.Message, workingDir string) error {
		return s.extractAfterTurn(ctx, messages)
	}
}

// extractAfterTurn is the internal entry point called after each complete
// conversation turn. It checks feature gates, then fires the extraction
// asynchronously with coalescing support.
func (s *System) extractAfterTurn(ctx context.Context, messages []message.Message) error {
	// Feature gate: check if extractMemories is enabled.
	if !IsExtractMemoriesEnabled() {
		if !s.state.HasLoggedGateFailure() {
			s.state.SetHasLoggedGateFailure(true)
			logger.DebugCF("extractmemories", "gate disabled", nil)
		}
		return nil
	}

	// Check auto-memory is enabled.
	if !IsAutoMemoryEnabled() {
		return nil
	}

	// Skip in remote mode.
	if IsRemoteMode() {
		return nil
	}

	// If an extraction is already in progress, skip this turn.
	// The in-progress run's cursor advance means the next extraction will
	// pick up new messages from after that point.
	if s.state.IsInProgress() {
		logger.DebugCF("extractmemories", "extraction in progress — coalescing", nil)
		return nil
	}
	s.state.SetInProgress(true)

	// Detach from the hook context so the background extraction isn't
	// canceled as soon as the post-turn hook returns. Use a background
	// context with its own timeout.
	s.inflight.Add(1)
	go func() {
		defer s.inflight.Done()
		defer s.state.SetInProgress(false)
		bgCtx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()
		s.runExtraction(bgCtx, messages, false)
	}()

	return nil
}

// DrainPendingExtraction waits for in-flight extractions to complete, with a
// soft timeout (milliseconds). Call before graceful shutdown so the forked
// agent completes before the process exits.
func (s *System) DrainPendingExtraction(timeoutMs int) {
	if timeoutMs <= 0 {
		timeoutMs = DefaultDrainTimeoutMs
	}
	done := make(chan struct{})
	go func() {
		s.inflight.Wait()
		close(done)
	}()
	select {
	case <-done:
		logger.DebugCF("extractmemories", "drain complete", nil)
	case <-time.After(time.Duration(timeoutMs) * time.Millisecond):
		logger.DebugCF("extractmemories", "drain timed out", map[string]any{
			"timeout_ms": timeoutMs,
		})
	}
}

// ResetForTesting resets the system state (used in tests).
func (s *System) ResetForTesting() {
	s.state.Reset()
}
