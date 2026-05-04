package rename

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

func TestSuggest_Success(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_RENAME_SUGGESTION", "1")
	mq := &mockQuerier{result: &haiku.QueryResult{Text: `{"name": "fix-login-bug"}`}}
	svc := NewService(mq)

	messages := []message.Message{
		{Role: message.RoleUser, Content: []message.ContentPart{{Type: "text", Text: "I need to fix the login bug"}}},
	}
	result, err := svc.Suggest(context.Background(), messages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "fix-login-bug" {
		t.Errorf("result = %q, want %q", result, "fix-login-bug")
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

func TestSuggest_FlagDisabled(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_RENAME_SUGGESTION", "0")
	mq := &mockQuerier{}
	svc := NewService(mq)

	messages := []message.Message{
		{Role: message.RoleUser, Content: []message.ContentPart{{Type: "text", Text: "hello"}}},
	}
	result, err := svc.Suggest(context.Background(), messages)
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

func TestSuggest_EmptyMessages(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_RENAME_SUGGESTION", "1")
	mq := &mockQuerier{}
	svc := NewService(mq)

	result, err := svc.Suggest(context.Background(), []message.Message{})
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

func TestSuggest_NoExtractableText(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_RENAME_SUGGESTION", "1")
	mq := &mockQuerier{}
	svc := NewService(mq)

	messages := []message.Message{
		{Role: message.RoleSystem, Content: []message.ContentPart{{Type: "text", Text: "system prompt"}}},
	}
	result, err := svc.Suggest(context.Background(), messages)
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

func TestSuggest_QuerierError(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_RENAME_SUGGESTION", "1")
	mq := &mockQuerier{err: errors.New("haiku overloaded")}
	svc := NewService(mq)

	messages := []message.Message{
		{Role: message.RoleUser, Content: []message.ContentPart{{Type: "text", Text: "hello"}}},
	}
	result, err := svc.Suggest(context.Background(), messages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("result = %q, want empty", result)
	}
}

func TestSuggest_QuerierReturnsNilResult(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_RENAME_SUGGESTION", "1")
	mq := &mockQuerier{result: nil}
	svc := NewService(mq)

	messages := []message.Message{
		{Role: message.RoleUser, Content: []message.ContentPart{{Type: "text", Text: "hello"}}},
	}
	result, err := svc.Suggest(context.Background(), messages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("result = %q, want empty", result)
	}
}

func TestSuggest_UnparseableJSON(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_RENAME_SUGGESTION", "1")
	mq := &mockQuerier{result: &haiku.QueryResult{Text: "not json"}}
	svc := NewService(mq)

	messages := []message.Message{
		{Role: message.RoleUser, Content: []message.ContentPart{{Type: "text", Text: "hello"}}},
	}
	result, err := svc.Suggest(context.Background(), messages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("result = %q, want empty", result)
	}
}

func TestSuggest_NilService(t *testing.T) {
	var svc *Service
	messages := []message.Message{
		{Role: message.RoleUser, Content: []message.ContentPart{{Type: "text", Text: "hello"}}},
	}
	result, err := svc.Suggest(context.Background(), messages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("result = %q, want empty", result)
	}
}

func TestSuggest_PackageLevel(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_RENAME_SUGGESTION", "1")
	mq := &mockQuerier{result: &haiku.QueryResult{Text: `{"name": "pkg-name"}`}}
	setCurrentService(NewService(mq))
	defer setCurrentService(nil)

	messages := []message.Message{
		{Role: message.RoleUser, Content: []message.ContentPart{{Type: "text", Text: "hello"}}},
	}
	result, err := Suggest(context.Background(), messages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "pkg-name" {
		t.Errorf("result = %q, want %q", result, "pkg-name")
	}
}

func TestSuggest_PackageLevelUninitialized(t *testing.T) {
	setCurrentService(nil)
	messages := []message.Message{
		{Role: message.RoleUser, Content: []message.ContentPart{{Type: "text", Text: "hello"}}},
	}
	result, err := Suggest(context.Background(), messages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("result = %q, want empty", result)
	}
}
