package microcompact

import (
	"math"
	"strings"
	"sync"
	"time"
)

// TIME_BASED_MC_CLEARED_MESSAGE is the marker string set in compacted tool_result
// content after time-based microcompact clears it. Aligns with TS
// TIME_BASED_MC_CLEARED_MESSAGE.
const TIME_BASED_MC_CLEARED_MESSAGE = "[Old tool result content cleared]"

// defaultBytesPerToken is the character-to-token heuristic ratio for rough estimation.
const defaultBytesPerToken = 4

// MicrocompactService provides time-based tool-result compaction for the engine loop.
// It content-clears expired tool results from user messages when the gap since the
// last assistant message exceeds a configurable threshold.
type MicrocompactService struct {
	mu    sync.Mutex
	state serviceState
}

type serviceState struct {
	// warningSuppressed tracks whether the compact warning has been suppressed
	// after a successful compaction.
	warningSuppressed bool
}

// NewMicrocompactService creates a new MicrocompactService with default state.
func NewMicrocompactService() *MicrocompactService {
	return &MicrocompactService{}
}

// EvaluateTimeBasedTrigger checks whether the time-based trigger should fire
// for this request. Returns the measured gap and active config when the trigger
// fires, or nil when it doesn't.
// Aligns with TS evaluateTimeBasedTrigger (microCompact.ts:422-444).
func (s *MicrocompactService) EvaluateTimeBasedTrigger(messages []Message, querySource string) *TimeBasedTriggerResult {
	cfg := DefaultTimeBasedMCConfig()

	if !cfg.Enabled {
		return nil
	}

	// Require a main-thread querySource. Subagents should not trigger time-based MC.
	if querySource == "" || !isMainThreadSource(querySource) {
		return nil
	}

	var lastAssistant *Message
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Type == "assistant" {
			lastAssistant = &messages[i]
			break
		}
	}
	if lastAssistant == nil || lastAssistant.Timestamp == "" {
		return nil
	}

	ts, err := time.Parse(time.RFC3339, lastAssistant.Timestamp)
	if err != nil {
		return nil
	}

	gapMinutes := time.Since(ts).Minutes()
	if gapMinutes < 0 || gapMinutes < float64(cfg.GapThresholdMinutes) {
		return nil
	}

	return &TimeBasedTriggerResult{
		GapMinutes: gapMinutes,
		Config:     cfg,
	}
}

// MicrocompactMessages orchestrates tool-result compaction for the given messages.
// In the Go implementation, only the time-based path is active (cached MC is ant-only).
// Aligns with TS microcompactMessages (microCompact.ts:253-293).
func (s *MicrocompactService) MicrocompactMessages(messages []Message, querySource string) MicrocompactResult {
	s.ClearCompactWarningSuppression()

	// Time-based trigger runs first and short-circuits.
	result := s.maybeTimeBasedMicrocompact(messages, querySource)
	if result != nil {
		return *result
	}

	// No cached MC path (ant-only). Return messages unchanged.
	return MicrocompactResult{Messages: messages}
}

// maybeTimeBasedMicrocompact implements the time-based compaction logic.
// Aligns with TS maybeTimeBasedMicrocompact (microCompact.ts:446-530).
func (s *MicrocompactService) maybeTimeBasedMicrocompact(messages []Message, querySource string) *MicrocompactResult {
	trigger := s.EvaluateTimeBasedTrigger(messages, querySource)
	if trigger == nil {
		return nil
	}

	compactableIDs := collectCompactableToolIDs(messages)

	// Floor at 1: clearing ALL results leaves the model with zero working context.
	keepRecent := int(math.Max(1, float64(trigger.Config.KeepRecent)))
	if keepRecent >= len(compactableIDs) {
		return nil
	}

	keepSet := make(map[string]bool)
	for _, id := range compactableIDs[len(compactableIDs)-keepRecent:] {
		keepSet[id] = true
	}

	clearSet := make(map[string]bool)
	for _, id := range compactableIDs {
		if !keepSet[id] {
			clearSet[id] = true
		}
	}

	if len(clearSet) == 0 {
		return nil
	}

	result := make([]Message, len(messages))
	for i, msg := range messages {
		if msg.Type != "user" || len(msg.Content) == 0 {
			result[i] = msg
			continue
		}

		touched := false
		newContent := make([]ContentPart, len(msg.Content))
		for j, block := range msg.Content {
			if block.Type == "tool_result" && clearSet[block.ToolUseID] && block.Text != TIME_BASED_MC_CLEARED_MESSAGE {
				newContent[j] = ContentPart{
					Type:      "tool_result",
					ToolUseID: block.ToolUseID,
					Text:      TIME_BASED_MC_CLEARED_MESSAGE,
				}
				touched = true
			} else {
				newContent[j] = block
			}
		}

		if touched {
			result[i] = Message{
				Type:      msg.Type,
				Content:   newContent,
				Timestamp: msg.Timestamp,
			}
		} else {
			result[i] = msg
		}
	}

	// Suppress warning after successful compaction.
	s.SuppressCompactWarning()

	return &MicrocompactResult{Messages: result}
}

// collectCompactableToolIDs walks assistant messages and collects tool_use IDs
// whose tool name is in COMPACTABLE_TOOLS, in encounter order.
func collectCompactableToolIDs(messages []Message) []string {
	var ids []string
	for _, msg := range messages {
		if msg.Type != "assistant" {
			continue
		}
		for _, block := range msg.Content {
			if block.Type == "tool_use" && CompactableToolSet[block.ToolName] {
				ids = append(ids, block.ToolUseID)
			}
		}
	}
	return ids
}

// isMainThreadSource returns true when the querySource originates from the
// main REPL thread (starts with "repl_main_thread"). Subagents use different
// querySource values and should not trigger time-based MC.
func isMainThreadSource(querySource string) bool {
	return strings.HasPrefix(querySource, "repl_main_thread")
}

// SuppressCompactWarning suppresses the compact warning. Called after successful compaction.
func (s *MicrocompactService) SuppressCompactWarning() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.warningSuppressed = true
}

// ClearCompactWarningSuppression clears the compact warning suppression.
// Called at start of new microcompact attempt.
func (s *MicrocompactService) ClearCompactWarningSuppression() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.warningSuppressed = false
}

// IsCompactWarningSuppressed reports whether the compact warning is currently suppressed.
func (s *MicrocompactService) IsCompactWarningSuppressed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state.warningSuppressed
}
