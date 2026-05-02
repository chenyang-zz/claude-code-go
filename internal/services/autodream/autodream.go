package autodream

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
	"github.com/sheepzhao/claude-code-go/internal/services/extractmemories"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const (
	// sessionScanIntervalMs is the minimum interval between session directory
	// scans. When the time-gate passes but the session-gate doesn't, the lock
	// mtime doesn't advance, so the time-gate keeps passing every turn.
	// This throttle prevents excessive directory scanning.
	sessionScanIntervalMs = 10 * 60 * 1000 // 10 minutes
)

// SubagentRunner executes a forked subagent with the given messages.
type SubagentRunner interface {
	Run(ctx context.Context, messages []message.Message) error
}

// System orchestrates the autoDream consolidation pipeline: time gate,
// session gate, lock acquisition, forked subagent execution, and progress
// tracking.
type System struct {
	runner      SubagentRunner
	projectRoot string

	// lastSessionScanAt records the last time the session directory was scanned.
	lastSessionScanAt int64

	mu sync.Mutex
}

// NewSystem creates a new autoDream system with the given runner and project
// root. Pass nil runner to skip subagent execution (gate checks and prompt
// building still work).
func NewSystem(runner SubagentRunner, projectRoot string) *System {
	return &System{
		runner:      runner,
		projectRoot: projectRoot,
	}
}

// isGateOpen checks whether the preconditions for autoDream are met.
// All checks are cheap.
func (s *System) isGateOpen() bool {
	if !isAutoDreamEnabled() {
		return false
	}
	if !extractmemories.IsAutoMemoryEnabled() {
		return false
	}
	if extractmemories.IsRemoteMode() {
		return false
	}
	return true
}

// RunAutoDream is the main entry point called after each conversation turn.
// It evaluates the three gates (time, session count, lock) and fires a
// forked subagent for memory consolidation when all pass.
func (s *System) RunAutoDream(ctx context.Context) error {
	if !s.isGateOpen() {
		return nil
	}

	cfg := getConfig()

	// Time gate: hours since last consolidation.
	lastAt, err := readLastConsolidatedAt(s.projectRoot)
	if err != nil {
		logger.DebugCF("autodream", "readLastConsolidatedAt failed", map[string]any{
			"error": err.Error(),
		})
		return nil
	}
	hoursSince := float64(time.Now().UnixMilli()-lastAt) / 3_600_000
	if lastAt > 0 && hoursSince < float64(cfg.MinHours) {
		return nil
	}

	// Scan throttle: avoid scanning the session directory every turn
	// when the time-gate passes but the session-gate doesn't.
	s.mu.Lock()
	sinceScanMs := time.Now().UnixMilli() - s.lastSessionScanAt
	if sinceScanMs < sessionScanIntervalMs && lastAt > 0 {
		s.mu.Unlock()
		logger.DebugCF("autodream", "scan throttle — time-gate passed but last scan was recent", map[string]any{
			"since_scan_secs": sinceScanMs / 1000,
		})
		return nil
	}
	s.lastSessionScanAt = time.Now().UnixMilli()
	s.mu.Unlock()

	// Session gate: enough sessions touched since last consolidation.
	sessionIds, err := listSessionsTouchedSince(s.projectRoot, lastAt)
	if err != nil {
		logger.DebugCF("autodream", "listSessionsTouchedSince failed", map[string]any{
			"error": err.Error(),
		})
		return nil
	}
	// Exclude the current session (its mtime is always recent).
	currentSession := getSessionID()
	filtered := make([]string, 0, len(sessionIds))
	for _, id := range sessionIds {
		if id != currentSession {
			filtered = append(filtered, id)
		}
	}
	if len(filtered) < cfg.MinSessions {
		logger.DebugCF("autodream", "skip — insufficient sessions since last consolidation", map[string]any{
			"session_count": len(filtered),
			"min_sessions":  cfg.MinSessions,
		})
		return nil
	}

	// If no runner is configured, skip lock acquisition and execution entirely.
	// The lock mtime is the consolidation gate — acquiring it without running
	// a subagent would delay/skip future real consolidations.
	if s.runner == nil {
		logger.DebugCF("autodream", "subagent runner is nil, skipping consolidation", nil)
		return nil
	}

	// Lock: ensure no other process is mid-consolidation.
	priorMtime, err := tryAcquireConsolidationLock(s.projectRoot)
	if err != nil {
		logger.DebugCF("autodream", "lock acquire failed", map[string]any{
			"error": err.Error(),
		})
		return nil
	}
	if priorMtime == -1 {
		// Lock held by another live process.
		return nil
	}

	logger.DebugCF("autodream", "firing consolidation", map[string]any{
		"hours_since":   fmt.Sprintf("%.1fh", hoursSince),
		"session_count": len(filtered),
	})

	// Build the session list for the prompt.
	var sessionList strings.Builder
	for _, id := range filtered {
		sessionList.WriteString(fmt.Sprintf("- %s\n", id))
	}

	// Build the consolidation prompt.
	memoryRoot := extractmemories.GetAutoMemPath(s.projectRoot)
	transcriptDir := filepath.Join(s.projectRoot, ".claude", "transcripts")
	extra := fmt.Sprintf(`

**Tool constraints for this run:** Bash is restricted to read-only commands (`+"`ls`, `find`, `grep`, `cat`, `stat`, `wc`, `head`, `tail`"+` and similar). Anything that writes, redirects to a file, or modifies state will be denied. Plan your exploration with this in mind — no need to probe.

Sessions since last consolidation (%d):
%s`, len(filtered), sessionList.String())

	prompt := buildConsolidationPrompt(memoryRoot, transcriptDir, extra)

	promptMsg := message.Message{
		Role: message.RoleUser,
		Content: []message.ContentPart{
			message.TextPart(prompt),
		},
	}
	if runErr := s.runner.Run(ctx, []message.Message{promptMsg}); runErr != nil {
		logger.WarnCF("autodream", "subagent execution failed", map[string]any{
			"error": runErr.Error(),
		})
		// Rollback: rewind mtime so time-gate passes again.
		rollbackConsolidationLock(s.projectRoot, priorMtime)
		return nil
	}

	logger.DebugCF("autodream", "consolidation completed", nil)
	return nil
}
