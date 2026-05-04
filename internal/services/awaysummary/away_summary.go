package awaysummary

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
	"github.com/sheepzhao/claude-code-go/internal/core/model"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const (
	// recentMessageWindow is the number of recent messages used for context
	// when generating an away summary. Mirrors TS RECENT_MESSAGE_WINDOW.
	recentMessageWindow = 30

	// memoryIndexName is the MEMORY.md index filename.
	memoryIndexName = "MEMORY.md"
)

// System orchestrates away summary generation triggered by user idle periods.
// It records post-turn activity timestamps and generates a brief summary
// prompt for a small/fast model when the idle threshold is exceeded.
type System struct {
	client     model.Client
	cfg        Config
	lastTurnAt time.Time
	memBaseDir string
	mu         sync.Mutex
}

// NewSystem creates a new AwaySummary system. Pass nil client if the model
// client is not yet available; call SetModelClient before the first turn.
func NewSystem(client model.Client, memBaseDir string, cfg Config) *System {
	return &System{
		client:     client,
		cfg:        cfg,
		memBaseDir: memBaseDir,
		lastTurnAt: time.Now(),
	}
}

// SetModelClient updates the model client used for summary generation.
func (s *System) SetModelClient(client model.Client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.client = client
}

// RecordActivity records the current time as the last activity timestamp.
// Called from the PostTurnHook after each turn completes.
func (s *System) RecordActivity() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastTurnAt = time.Now()
}

// ShouldGenerate reports whether the idle threshold has been exceeded
// since the last recorded activity.
func (s *System) ShouldGenerate() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return time.Since(s.lastTurnAt) >= s.cfg.IdleThreshold
}

// Generate generates an away summary from the recent conversation messages.
// Returns the summary text. Returns empty string if messages is empty or
// generation fails (errors are logged, not surfaced).
func (s *System) Generate(ctx context.Context, messages []message.Message) (string, error) {
	if len(messages) == 0 {
		return "", nil
	}

	s.mu.Lock()
	client := s.client
	cfg := s.cfg
	memBaseDir := s.memBaseDir
	s.mu.Unlock()

	if client == nil {
		logger.DebugCF("awaysummary", "skipping generate: model client not set", nil)
		return "", nil
	}

	recent := messages
	if len(recent) > cfg.MaxMessages {
		recent = recent[len(recent)-cfg.MaxMessages:]
	}

	memoryContent := readMemoryContent(memBaseDir)
	prompt := buildPrompt(memoryContent)

	userMsg := message.Message{
		Role: message.RoleUser,
		Content: []message.ContentPart{
			message.TextPart(prompt),
		},
	}

	reqMessages := make([]message.Message, 0, len(recent)+1)
	reqMessages = append(reqMessages, recent...)
	reqMessages = append(reqMessages, userMsg)

	req := model.Request{
		Model:    cfg.Model,
		Messages: reqMessages,
	}

	stream, err := client.Stream(ctx, req)
	if err != nil {
		logger.DebugCF("awaysummary", "stream failed", map[string]any{
			"error": err.Error(),
		})
		return "", fmt.Errorf("away summary: stream failed: %w", err)
	}

	var text strings.Builder
	for event := range stream {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}

		switch event.Type {
		case model.EventTypeTextDelta:
			text.WriteString(event.Text)
		case model.EventTypeError:
			logger.DebugCF("awaysummary", "model stream error", map[string]any{
				"error": event.Error,
			})
			return "", fmt.Errorf("away summary: model error: %s", event.Error)
		case model.EventTypeDone:
			// Normal stream completion — fall through to return.
		}
	}

	return strings.TrimSpace(text.String()), nil
}

// buildPrompt constructs the away summary prompt with optional memory context.
// Mirrors TS buildAwaySummaryPrompt.
func buildPrompt(memoryContent string) string {
	var b strings.Builder
	if memoryContent != "" {
		b.WriteString("Session memory (broader context):\n")
		b.WriteString(memoryContent)
		b.WriteString("\n\n")
	}
	b.WriteString("The user stepped away and is coming back. Write exactly 1-3 short sentences. Start by stating the high-level task — what they are building or debugging, not implementation details. Next: the concrete next step. Skip status reports and commit recaps.")
	return b.String()
}

// readMemoryContent reads the combined memory content from the memory directory.
// Returns the MEMORY.md index content and a summary of memory files.
// Mirrors TS getSessionMemoryContent semantics — returns empty string if files
// cannot be read.
func readMemoryContent(memBaseDir string) string {
	if memBaseDir == "" {
		return ""
	}

	indexPath := filepath.Join(memBaseDir, memoryIndexName)
	data, err := os.ReadFile(indexPath)
	if err != nil {
		return ""
	}

	content := strings.TrimSpace(string(data))
	if len(content) > 2000 {
		content = content[:2000]
	}
	return content
}
