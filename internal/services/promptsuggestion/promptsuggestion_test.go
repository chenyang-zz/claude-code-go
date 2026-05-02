package promptsuggestion

import (
	"context"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
)

func TestNewSuggester(t *testing.T) {
	s := NewSuggester(nil)
	if s == nil {
		t.Fatal("expected non-nil Suggester")
	}
}

func TestSuggester_TryGenerate_Basic(t *testing.T) {
	s := NewSuggester(nil)
	messages := []message.Message{
		{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("hello")}},
		{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("hi")}},
		{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("how are you")}},
		{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("good")}},
	}

	outcome := s.TryGenerate(context.Background(), messages)
	if outcome.Suggestion == nil {
		t.Fatal("expected non-nil suggestion when runner is nil")
	}
	if outcome.Suggestion.Text != "placeholder suggestion" {
		t.Errorf("expected placeholder suggestion, got %q", outcome.Suggestion.Text)
	}
	if outcome.Suggestion.PromptID != PromptVariantUserIntent {
		t.Errorf("expected PromptVariantUserIntent, got %q", outcome.Suggestion.PromptID)
	}
}

func TestSuggester_Abort(t *testing.T) {
	s := NewSuggester(nil)
	// Abort should not panic even when nothing is running.
	s.Abort()
}

// mockRunner is a test double for SubagentRunner.
type mockRunner struct {
	runCalled bool
	ctx       context.Context
	messages  []message.Message
}

func (m *mockRunner) Run(ctx context.Context, messages []message.Message) error {
	m.runCalled = true
	m.ctx = ctx
	m.messages = messages
	return nil
}

func TestSuggester_TryGenerate_AbortsPrevious(t *testing.T) {
	runner := &mockRunner{}
	s := NewSuggester(runner)
	messages := []message.Message{
		{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("hello")}},
		{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("hi")}},
		{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("how are you")}},
		{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("good")}},
	}

	// First call starts a generation.
	_ = s.TryGenerate(context.Background(), messages)

	// Second call should abort the first.
	_ = s.TryGenerate(context.Background(), messages)

	if !runner.runCalled {
		t.Error("expected runner.Run to be called")
	}
}

func TestGetSuppressReason_EarlyConversation(t *testing.T) {
	messages := []message.Message{
		{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("hello")}},
		{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("hi")}},
	}

	reason := getSuppressReason(messages)
	if reason != SuppressEarlyConversation {
		t.Errorf("expected SuppressEarlyConversation, got %q", reason)
	}
}

func TestGetSuppressReason_LastResponseError(t *testing.T) {
	messages := []message.Message{
		{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("hello")}},
		{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("hi")}},
		{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("how are you")}},
		{Role: message.RoleAssistant, Content: []message.ContentPart{}},
	}

	reason := getSuppressReason(messages)
	if reason != SuppressLastResponseError {
		t.Errorf("expected SuppressLastResponseError, got %q", reason)
	}
}

func TestShouldFilter_Empty(t *testing.T) {
	filtered, reason := shouldFilter("")
	if !filtered {
		t.Error("expected empty string to be filtered")
	}
	if reason != SuppressEmpty {
		t.Errorf("expected SuppressEmpty, got %q", reason)
	}
}

func TestShouldFilter_TooLong(t *testing.T) {
	suggestion := "this is a very long suggestion that exceeds one hundred characters in total length for sure"
	filtered, reason := shouldFilter(suggestion)
	if !filtered {
		t.Error("expected long suggestion to be filtered")
	}
	if reason != SuppressFiltered {
		t.Errorf("expected SuppressFiltered, got %q", reason)
	}
}
