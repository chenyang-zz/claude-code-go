package agentsummary

import (
	"context"
	"sync"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
	"github.com/sheepzhao/claude-code-go/internal/runtime/engine"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// AgentSummarizer manages periodic agent progress summarization.
// Mirrors the TS startAgentSummarization function in agentSummary.ts.
// It uses a recursive timer pattern (complete-then-schedule) to avoid
// overlapping summaries, and uses the forked agent infrastructure (RunForked)
// to generate 3-5 word progress summaries.
type AgentSummarizer struct {
	taskID     string
	agentID    string
	params     engine.CacheSafeParams
	store      *SummaryStore
	runForked  func(context.Context, engine.ForkedAgentParams) (*engine.ForkedAgentResult, error)
	getMsgs    GetMessagesFunc
	cfg        SummaryConfig
	ctx        context.Context
	cancel     context.CancelFunc
	prevSum    string
	mu         sync.Mutex
	stopped    bool
	timer      *time.Timer
	wg         sync.WaitGroup
}

// StartAgentSummarization starts periodic summarization for an agent.
// It returns a StopFunc that stops the summarization and cleans up resources.
//
// Parameters:
//   - parentCtx: parent context; summarization stops when this is cancelled.
//   - taskID: the task identifier for this agent.
//   - agentID: the agent identifier (for logging).
//   - cacheSafeParams: cache-critical parameters from the parent agent.
//   - store: the shared summary store to write summaries to.
//   - runForked: the forked agent execution function (typically engine.RunForked).
//   - getMessages: returns the current live message history for this agent.
//   - opts: optional config options (e.g. WithInterval).
//
// Mirrors the TS startAgentSummarization signature but replaces the React
// setAppState callback with a SummaryStore, and replaces the implicit
// getAgentTranscript with an explicit getMessages callback.
func StartAgentSummarization(
	parentCtx context.Context,
	taskID string,
	agentID string,
	cacheSafeParams engine.CacheSafeParams,
	store *SummaryStore,
	runForked func(context.Context, engine.ForkedAgentParams) (*engine.ForkedAgentResult, error),
	getMessages GetMessagesFunc,
	opts ...ConfigOption,
) StopFunc {
	ctx, cancel := context.WithCancel(parentCtx)

	s := &AgentSummarizer{
		taskID:    taskID,
		agentID:   agentID,
		params:    cacheSafeParams,
		store:     store,
		runForked: runForked,
		getMsgs:   getMessages,
		cfg:       DefaultSummaryConfig(),
		ctx:       ctx,
		cancel:    cancel,
	}

	for _, opt := range opts {
		opt(&s.cfg)
	}

	s.scheduleNext()

	logger.DebugCF("agentsummary", "started agent summarization", map[string]any{
		"task_id":  taskID,
		"agent_id": agentID,
		"interval": s.cfg.Interval.String(),
	})

	return func() {
		s.mu.Lock()
		defer s.mu.Unlock()

		if s.stopped {
			return
		}
		s.stopped = true
		s.cancel()

		if s.timer != nil {
			s.timer.Stop()
			s.timer = nil
		}

		s.store.Delete(taskID)

		logger.DebugCF("agentsummary", "stopped agent summarization", map[string]any{
			"task_id":  taskID,
			"agent_id": agentID,
		})
	}
}

// scheduleNext schedules the next summary run after the configured interval.
// Mirrors the TS scheduleNext function.
func (s *AgentSummarizer) scheduleNext() {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return
	}
	s.timer = time.AfterFunc(s.cfg.Interval, func() {
		s.runSummary()
	})
	s.mu.Unlock()
}

// runSummary executes one summary generation cycle.
// Mirrors the TS runSummary async function.
func (s *AgentSummarizer) runSummary() {
	// Prevent overlapping summaries: if a summary is already running,
	// scheduleNext in the finally block of the previous run handles it.
	s.wg.Add(1)
	defer s.wg.Done()

	// Check if stopped.
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return
	}
	taskID := s.taskID
	agentID := s.agentID
	params := s.params
	runForked := s.runForked
	getMsgs := s.getMsgs
	prevSum := s.prevSum
	s.mu.Unlock()

	logger.DebugCF("agentsummary", "timer fired for agent", map[string]any{
		"agent_id": agentID,
		"task_id":  taskID,
	})

	// Get current messages from the agent's live history.
	messages := getMsgs()
	if len(messages) < 3 {
		logger.DebugCF("agentsummary", "skipping summary: not enough messages", map[string]any{
			"task_id":  taskID,
			"count":    len(messages),
		})
		// Schedule next attempt in the finally block.
		s.scheduleNext()
		return
	}

	// Filter incomplete tool calls (assistant tool_use without matching tool_result).
	cleanMessages := FilterIncompleteToolCalls(messages)

	// Build fork params with current messages.
	forkParams := params
	forkParams.Messages = cleanMessages

	// Build the summary prompt as a user message.
	promptText := BuildSummaryPrompt(prevSum)
	promptMsg := message.Message{
		Role: message.RoleUser,
		Content: []message.ContentPart{
			message.TextPart(promptText),
		},
	}

	// Create a cancelable context for this summary cycle.
	summaryCtx, summaryCancel := context.WithCancel(s.ctx)
	defer summaryCancel()

	// Execute the forked agent request.
	result, err := runForked(summaryCtx, engine.ForkedAgentParams{
		PromptMessages:  []message.Message{promptMsg},
		CacheSafeParams: forkParams,
		CanUseTool:      engine.DenyAllTools,
		ForkLabel:       "agent_summary",
		MaxTurns:        1,
		SkipTranscript:  true,
	})
	if err != nil {
		// Check if context was cancelled (stopped).
		if s.isStopped() {
			return
		}
		logger.DebugCF("agentsummary", "forked agent error", map[string]any{
			"task_id": taskID,
			"error":   err.Error(),
		})
		s.scheduleNext()
		return
	}

	if s.isStopped() {
		return
	}

	// Extract summary text from the result.
	summaryText := extractSummary(result.Messages)
	if summaryText != "" {
		s.mu.Lock()
		s.prevSum = summaryText
		s.mu.Unlock()

		s.store.Store(taskID, summaryText)

		logger.DebugCF("agentsummary", "summary result", map[string]any{
			"task_id": taskID,
			"summary": summaryText,
		})
	}

	// Schedule the next summary run.
	s.scheduleNext()
}

// isStopped returns true if the summarizer has been stopped.
func (s *AgentSummarizer) isStopped() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stopped
}

// extractSummary extracts the text content from the last assistant message
// in the given messages, skipping API error messages.
func extractSummary(messages []message.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role != message.RoleAssistant {
			continue
		}
		for _, part := range messages[i].Content {
			if part.Type == "text" && part.Text != "" {
				return part.Text
			}
		}
	}
	return ""
}

// ConfigOption is a functional option for configuring AgentSummarizer behavior.
type ConfigOption func(*SummaryConfig)

// WithInterval sets the summary generation interval.
func WithInterval(d time.Duration) ConfigOption {
	return func(c *SummaryConfig) {
		c.Interval = d
	}
}

// FilterIncompleteToolCalls removes assistant messages that contain tool_use
// blocks without corresponding tool_result blocks in subsequent user messages.
// Mirrors the TS filterIncompleteToolCalls function.
func FilterIncompleteToolCalls(messages []message.Message) []message.Message {
	if len(messages) == 0 {
		return messages
	}

	// Collect all tool_use IDs that have matching tool_results.
	completedToolIDs := make(map[string]bool)
	for _, msg := range messages {
		if msg.Role == message.RoleUser {
			for _, part := range msg.Content {
				if part.ToolUseID != "" {
					completedToolIDs[part.ToolUseID] = true
				}
			}
		}
	}

	// Filter out assistant messages whose tool_use blocks lack results.
	result := make([]message.Message, 0, len(messages))
	for _, msg := range messages {
		if msg.Role == message.RoleAssistant {
			hasMissingResult := false
			for _, part := range msg.Content {
				if part.ToolUseID != "" && !completedToolIDs[part.ToolUseID] {
					hasMissingResult = true
					break
				}
			}
			if hasMissingResult {
				continue
			}
		}
		result = append(result, msg)
	}

	return result
}
