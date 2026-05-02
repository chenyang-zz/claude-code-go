package promptsuggestion

import (
	"context"
	"strings"
	"sync"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
)

// SuggestionResult stores the result of a single suggestion generation.
type SuggestionResult struct {
	Text                string
	PromptID            PromptVariant
	GenerationRequestID string
}

// SuppressReason describes why a suggestion was suppressed.
type SuppressReason string

const (
	// SuppressDisabled indicates the feature is disabled.
	SuppressDisabled SuppressReason = "disabled"
	// SuppressNonInteractive indicates the session is non-interactive.
	SuppressNonInteractive SuppressReason = "non_interactive"
	// SuppressTeammate indicates the current agent is a teammate.
	SuppressTeammate SuppressReason = "teammate"
	// SuppressEarlyConversation indicates the conversation is too short.
	SuppressEarlyConversation SuppressReason = "early_conversation"
	// SuppressLastResponseError indicates the last assistant response was empty or errored.
	SuppressLastResponseError SuppressReason = "last_response_error"
	// SuppressEmpty indicates the suggestion is empty.
	SuppressEmpty SuppressReason = "empty"
	// SuppressFiltered indicates the suggestion was filtered by content rules.
	SuppressFiltered SuppressReason = "filtered"
)

// Outcome describes the result of a TryGenerate call.
type Outcome struct {
	Suggestion *SuggestionResult
	Suppress   SuppressReason
	Error      error
}

// SubagentRunner is the interface for executing a forked subagent.
type SubagentRunner interface {
	Run(ctx context.Context, messages []message.Message) error
}

// Suggester maintains a singleton abort controller and is responsible for
// generating prompt suggestions.
type Suggester struct {
	mu     sync.Mutex
	cancel context.CancelFunc
	runner SubagentRunner
}

// NewSuggester creates a new Suggester instance.
func NewSuggester(runner SubagentRunner) *Suggester {
	return &Suggester{
		runner: runner,
	}
}

// TryGenerate attempts to generate a suggestion.
// It is the main entry point.
//
// Flow:
//  1. Abort any ongoing generation.
//  2. Check suppress reasons.
//  3. If runner is nil, return a placeholder suggestion.
//  4. Otherwise, execute the fork agent via runner (placeholder, logs only).
func (s *Suggester) TryGenerate(ctx context.Context, messages []message.Message) Outcome {
	s.Abort()

	if reason := getSuppressReason(messages); reason != "" {
		return Outcome{Suppress: reason}
	}

	if s.runner == nil {
		return Outcome{
			Suggestion: &SuggestionResult{
				Text:     "placeholder suggestion",
				PromptID: GetPromptVariant(),
			},
		}
	}

	// Placeholder: fork agent execution via runner.
	// For now, just log that we would run the subagent.
	ctx, cancel := context.WithCancel(ctx)
	s.mu.Lock()
	s.cancel = cancel
	s.mu.Unlock()

	_ = s.runner.Run(ctx, messages)

	return Outcome{
		Suggestion: &SuggestionResult{
			Text:     "placeholder suggestion",
			PromptID: GetPromptVariant(),
		},
	}
}

// Abort aborts the current in-progress suggestion generation.
func (s *Suggester) Abort() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
}

// shouldFilter checks whether a suggestion should be filtered.
// Rules: empty, done, too_long (>100 chars), too_many_words (>12 words),
// multiple_sentences.
func shouldFilter(suggestion string) (bool, SuppressReason) {
	if suggestion == "" {
		return true, SuppressEmpty
	}

	if strings.ToLower(suggestion) == "done" {
		return true, SuppressFiltered
	}

	if len(suggestion) > 100 {
		return true, SuppressFiltered
	}

	words := strings.Fields(suggestion)
	if len(words) > 12 {
		return true, SuppressFiltered
	}

	if strings.Contains(suggestion, ".") && !strings.HasSuffix(suggestion, ".") {
		return true, SuppressFiltered
	}

	return false, ""
}

// getSuppressReason detects the suppress reason for the given messages.
// Returns an empty string if no suppression is needed.
//
// Current simplified implementation:
//   - messages length < 4 (< 2 assistant turns) → early_conversation
//   - last message is assistant and is empty/errored → last_response_error
func getSuppressReason(messages []message.Message) SuppressReason {
	if len(messages) < 4 {
		return SuppressEarlyConversation
	}

	if len(messages) > 0 {
		last := messages[len(messages)-1]
		if last.Role == message.RoleAssistant {
			if len(last.Content) == 0 {
				return SuppressLastResponseError
			}
			for _, part := range last.Content {
				if part.Type == "text" && part.Text == "" {
					return SuppressLastResponseError
				}
				if part.IsError {
					return SuppressLastResponseError
				}
			}
		}
	}

	return ""
}
