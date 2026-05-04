package sessiontitle

import (
	"context"
	"errors"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
	"github.com/sheepzhao/claude-code-go/internal/services/haiku"
)

// mockQuerier implements haiku.Querier for tests.
type mockQuerier struct {
	result *haiku.QueryResult
	err    error
	calls  []haiku.QueryParams
}

func (m *mockQuerier) Query(ctx context.Context, params haiku.QueryParams) (*haiku.QueryResult, error) {
	m.calls = append(m.calls, params)
	if m.err != nil {
		return nil, m.err
	}
	return m.result, nil
}

func TestGenerate_Success(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_SESSION_TITLE", "1")
	mq := &mockQuerier{result: &haiku.QueryResult{Text: `{"title": "Fix login button"}`}}
	svc := NewService(mq)

	result, err := svc.Generate(context.Background(), "User asked to fix the login button on mobile")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Fix login button" {
		t.Errorf("result = %q, want %q", result, "Fix login button")
	}
	if len(mq.calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(mq.calls))
	}
	if !mq.calls[0].EnablePromptCaching {
		t.Error("expected prompt caching enabled")
	}
	if mq.calls[0].QuerySource != querySource {
		t.Errorf("query_source = %q, want %q", mq.calls[0].QuerySource, querySource)
	}
}

func TestGenerate_FlagDisabled(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_SESSION_TITLE", "0")
	mq := &mockQuerier{}
	svc := NewService(mq)

	result, err := svc.Generate(context.Background(), "some description")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("result = %q, want empty", result)
	}
	if len(mq.calls) != 0 {
		t.Error("expected no querier calls")
	}
}

func TestGenerate_EmptyDescription(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_SESSION_TITLE", "1")
	mq := &mockQuerier{}
	svc := NewService(mq)

	result, err := svc.Generate(context.Background(), "   ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("result = %q, want empty", result)
	}
	if len(mq.calls) != 0 {
		t.Error("expected no querier calls")
	}
}

func TestGenerate_QuerierError(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_SESSION_TITLE", "1")
	mq := &mockQuerier{err: errors.New("haiku overloaded")}
	svc := NewService(mq)

	result, err := svc.Generate(context.Background(), "some description")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("result = %q, want empty", result)
	}
}

func TestGenerate_QuerierReturnsNilResult(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_SESSION_TITLE", "1")
	mq := &mockQuerier{result: nil}
	svc := NewService(mq)

	result, err := svc.Generate(context.Background(), "some description")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("result = %q, want empty", result)
	}
}

func TestGenerate_QuerierReturnsEmptyText(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_SESSION_TITLE", "1")
	mq := &mockQuerier{result: &haiku.QueryResult{Text: "   "}}
	svc := NewService(mq)

	result, err := svc.Generate(context.Background(), "some description")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("result = %q, want empty", result)
	}
}

func TestGenerate_UnparseableJSON(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_SESSION_TITLE", "1")
	mq := &mockQuerier{result: &haiku.QueryResult{Text: "not json"}}
	svc := NewService(mq)

	result, err := svc.Generate(context.Background(), "some description")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("result = %q, want empty", result)
	}
}

func TestGenerate_EmptyTitleField(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_SESSION_TITLE", "1")
	mq := &mockQuerier{result: &haiku.QueryResult{Text: `{"title": "  "}`}}
	svc := NewService(mq)

	result, err := svc.Generate(context.Background(), "some description")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("result = %q, want empty", result)
	}
}

func TestGenerate_NilService(t *testing.T) {
	var svc *Service
	result, err := svc.Generate(context.Background(), "some description")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("result = %q, want empty", result)
	}
}

func TestGenerate_PackageLevel(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_SESSION_TITLE", "1")
	mq := &mockQuerier{result: &haiku.QueryResult{Text: `{"title": "pkg-title"}`}}
	setCurrentService(NewService(mq))
	defer setCurrentService(nil)

	result, err := Generate(context.Background(), "some description")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "pkg-title" {
		t.Errorf("result = %q, want %q", result, "pkg-title")
	}
}

func TestGenerate_PackageLevelUninitialized(t *testing.T) {
	setCurrentService(nil)
	result, err := Generate(context.Background(), "some description")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("result = %q, want empty", result)
	}
}

func TestExtractConversationText(t *testing.T) {
	messages := []message.Message{
		{
			Role:    message.RoleUser,
			Content: []message.ContentPart{{Type: "text", Text: "Hello world"}},
		},
		{
			Role:    message.RoleAssistant,
			Content: []message.ContentPart{{Type: "text", Text: "Hi there"}},
		},
	}
	result := ExtractConversationText(messages)
	expected := "Hello world\nHi there"
	if result != expected {
		t.Errorf("result = %q, want %q", result, expected)
	}
}

func TestExtractConversationText_SkipsMeta(t *testing.T) {
	messages := []message.Message{
		{
			Role:    message.RoleUser,
			Content: []message.ContentPart{{Type: "text", Text: "Hello", IsMeta: true}},
		},
		{
			Role:    message.RoleUser,
			Content: []message.ContentPart{{Type: "text", Text: "Real message"}},
		},
	}
	result := ExtractConversationText(messages)
	expected := "Real message"
	if result != expected {
		t.Errorf("result = %q, want %q", result, expected)
	}
}

func TestExtractConversationText_SkipsNonText(t *testing.T) {
	messages := []message.Message{
		{
			Role:    message.RoleUser,
			Content: []message.ContentPart{{Type: "image", MediaType: "image/png", Base64Data: "abc"}},
		},
		{
			Role:    message.RoleUser,
			Content: []message.ContentPart{{Type: "text", Text: "Only text"}},
		},
	}
	result := ExtractConversationText(messages)
	expected := "Only text"
	if result != expected {
		t.Errorf("result = %q, want %q", result, expected)
	}
}

func TestExtractConversationText_Truncates(t *testing.T) {
	longText := make([]byte, maxConversationText+100)
	for i := range longText {
		longText[i] = 'a'
	}
	messages := []message.Message{
		{
			Role:    message.RoleUser,
			Content: []message.ContentPart{{Type: "text", Text: string(longText)}},
		},
	}
	result := ExtractConversationText(messages)
	if len(result) != maxConversationText {
		t.Errorf("len = %d, want %d", len(result), maxConversationText)
	}
}
