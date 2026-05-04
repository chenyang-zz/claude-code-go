package toolusesummary

import (
	"context"
	"errors"
	"testing"

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
	t.Setenv("CLAUDE_FEATURE_TOOL_USE_SUMMARY", "1")
	mq := &mockQuerier{result: &haiku.QueryResult{Text: "Reading files"}}
	svc := NewService(mq)

	result, err := svc.Generate(context.Background(), SummaryParams{
		Tools: []ToolInfo{
			{Name: "Read", Input: map[string]any{"file_path": "/tmp/a"}, Output: "content"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Reading files" {
		t.Errorf("result = %q, want %q", result, "Reading files")
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
	t.Setenv("CLAUDE_FEATURE_TOOL_USE_SUMMARY", "0")
	mq := &mockQuerier{}
	svc := NewService(mq)

	result, err := svc.Generate(context.Background(), SummaryParams{
		Tools: []ToolInfo{{Name: "Read", Input: "x", Output: "y"}},
	})
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

func TestGenerate_EmptyTools(t *testing.T) {
	mq := &mockQuerier{}
	svc := NewService(mq)

	result, err := svc.Generate(context.Background(), SummaryParams{Tools: []ToolInfo{}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("result = %q, want empty", result)
	}
}

func TestGenerate_NilTools(t *testing.T) {
	mq := &mockQuerier{}
	svc := NewService(mq)

	result, err := svc.Generate(context.Background(), SummaryParams{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("result = %q, want empty", result)
	}
}

func TestGenerate_QuerierError(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_TOOL_USE_SUMMARY", "1")
	mq := &mockQuerier{err: errors.New("haiku overloaded")}
	svc := NewService(mq)

	result, err := svc.Generate(context.Background(), SummaryParams{
		Tools: []ToolInfo{{Name: "Read", Input: "x", Output: "y"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("result = %q, want empty", result)
	}
}

func TestGenerate_QuerierReturnsNilResult(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_TOOL_USE_SUMMARY", "1")
	mq := &mockQuerier{result: nil}
	svc := NewService(mq)

	result, err := svc.Generate(context.Background(), SummaryParams{
		Tools: []ToolInfo{{Name: "Read", Input: "x", Output: "y"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("result = %q, want empty", result)
	}
}

func TestGenerate_QuerierReturnsEmptyText(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_TOOL_USE_SUMMARY", "1")
	mq := &mockQuerier{result: &haiku.QueryResult{Text: "   "}}
	svc := NewService(mq)

	result, err := svc.Generate(context.Background(), SummaryParams{
		Tools: []ToolInfo{{Name: "Read", Input: "x", Output: "y"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("result = %q, want empty", result)
	}
}

func TestGenerate_WithLastAssistantText(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_TOOL_USE_SUMMARY", "1")
	mq := &mockQuerier{result: &haiku.QueryResult{Text: "summary"}}
	svc := NewService(mq)

	_, _ = svc.Generate(context.Background(), SummaryParams{
		Tools:             []ToolInfo{{Name: "Read", Input: "x", Output: "y"}},
		LastAssistantText: "Please read the file",
	})
	if len(mq.calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(mq.calls))
	}
	if mq.calls[0].UserPrompt == "" {
		t.Error("expected non-empty user prompt")
	}
}

func TestGenerate_NilService(t *testing.T) {
	var svc *Service
	result, err := svc.Generate(context.Background(), SummaryParams{
		Tools: []ToolInfo{{Name: "Read", Input: "x", Output: "y"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("result = %q, want empty", result)
	}
}

func TestGenerate_PackageLevel(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_TOOL_USE_SUMMARY", "1")
	mq := &mockQuerier{result: &haiku.QueryResult{Text: "pkg-summary"}}
	setCurrentService(NewService(mq))
	defer setCurrentService(nil)

	result, err := Generate(context.Background(), SummaryParams{
		Tools: []ToolInfo{{Name: "Read", Input: "x", Output: "y"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "pkg-summary" {
		t.Errorf("result = %q, want %q", result, "pkg-summary")
	}
}

func TestGenerate_PackageLevelUninitialized(t *testing.T) {
	setCurrentService(nil)
	result, err := Generate(context.Background(), SummaryParams{
		Tools: []ToolInfo{{Name: "Read", Input: "x", Output: "y"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("result = %q, want empty", result)
	}
}

func TestGenerate_IsNonInteractiveSession(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_TOOL_USE_SUMMARY", "1")
	mq := &mockQuerier{result: &haiku.QueryResult{Text: "summary"}}
	svc := NewService(mq)

	_, _ = svc.Generate(context.Background(), SummaryParams{
		Tools:                   []ToolInfo{{Name: "Read", Input: "x", Output: "y"}},
		IsNonInteractiveSession: true,
	})
	if len(mq.calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(mq.calls))
	}
	if !mq.calls[0].IsNonInteractiveSession {
		t.Error("expected IsNonInteractiveSession forwarded")
	}
}
