package datetimeparser

import (
	"context"
	"errors"
	"strings"
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

func TestParse_Success(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_DATETIME_PARSER", "1")
	mq := &mockQuerier{result: &haiku.QueryResult{Text: "2025-10-15T15:00:00-07:00"}}
	svc := NewService(mq)

	result, err := svc.Parse(context.Background(), "tomorrow at 3pm", "date-time")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}
	if result.Value != "2025-10-15T15:00:00-07:00" {
		t.Errorf("value = %q, want %q", result.Value, "2025-10-15T15:00:00-07:00")
	}
	if len(mq.calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(mq.calls))
	}
	if mq.calls[0].EnablePromptCaching {
		t.Error("expected prompt caching disabled")
	}
	if mq.calls[0].QuerySource != querySource {
		t.Errorf("query_source = %q, want %q", mq.calls[0].QuerySource, querySource)
	}
}

func TestParse_SuccessDateOnly(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_DATETIME_PARSER", "1")
	mq := &mockQuerier{result: &haiku.QueryResult{Text: "2025-10-15"}}
	svc := NewService(mq)

	result, err := svc.Parse(context.Background(), "next Monday", "date")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}
	if result.Value != "2025-10-15" {
		t.Errorf("value = %q, want %q", result.Value, "2025-10-15")
	}
}

func TestParse_FlagDisabled(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_DATETIME_PARSER", "0")
	mq := &mockQuerier{}
	svc := NewService(mq)

	result, err := svc.Parse(context.Background(), "tomorrow", "date")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected failure")
	}
	if len(mq.calls) != 0 {
		t.Error("expected no querier calls")
	}
}

func TestParse_EmptyInput(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_DATETIME_PARSER", "1")
	mq := &mockQuerier{}
	svc := NewService(mq)

	result, err := svc.Parse(context.Background(), "   ", "date")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected failure")
	}
	if len(mq.calls) != 0 {
		t.Error("expected no querier calls")
	}
}

func TestParse_InvalidFormat(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_DATETIME_PARSER", "1")
	mq := &mockQuerier{}
	svc := NewService(mq)

	result, err := svc.Parse(context.Background(), "tomorrow", "datetime")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected failure for invalid format")
	}
	if len(mq.calls) != 0 {
		t.Error("expected no querier calls")
	}
}

func TestParse_InputWithQuotes(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_DATETIME_PARSER", "1")
	mq := &mockQuerier{result: &haiku.QueryResult{Text: "2025-01-01"}}
	svc := NewService(mq)

	result, err := svc.Parse(context.Background(), `tomorrow at "3pm"`, "date")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}
	if len(mq.calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(mq.calls))
	}
	// Verify quotes are escaped in the user prompt.
	if !strings.Contains(mq.calls[0].UserPrompt, `\"3pm\"`) {
		t.Errorf("expected escaped quotes in prompt, got: %q", mq.calls[0].UserPrompt)
	}
}

func TestParse_QuerierError(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_DATETIME_PARSER", "1")
	mq := &mockQuerier{err: errors.New("haiku overloaded")}
	svc := NewService(mq)

	result, err := svc.Parse(context.Background(), "tomorrow", "date")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected failure")
	}
}

func TestParse_QuerierReturnsNilResult(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_DATETIME_PARSER", "1")
	mq := &mockQuerier{result: nil}
	svc := NewService(mq)

	result, err := svc.Parse(context.Background(), "tomorrow", "date")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected failure")
	}
}

func TestParse_InvalidResponse(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_DATETIME_PARSER", "1")
	mq := &mockQuerier{result: &haiku.QueryResult{Text: "INVALID"}}
	svc := NewService(mq)

	result, err := svc.Parse(context.Background(), "gibberish", "date")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected failure")
	}
}

func TestParse_EmptyResponse(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_DATETIME_PARSER", "1")
	mq := &mockQuerier{result: &haiku.QueryResult{Text: "   "}}
	svc := NewService(mq)

	result, err := svc.Parse(context.Background(), "tomorrow", "date")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected failure")
	}
}

func TestParse_NoYearPrefix(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_DATETIME_PARSER", "1")
	mq := &mockQuerier{result: &haiku.QueryResult{Text: "Oct 15"}}
	svc := NewService(mq)

	result, err := svc.Parse(context.Background(), "tomorrow", "date")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected failure")
	}
}

func TestParse_NilService(t *testing.T) {
	var svc *Service
	result, err := svc.Parse(context.Background(), "tomorrow", "date")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected failure")
	}
}

func TestParse_PackageLevel(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_DATETIME_PARSER", "1")
	mq := &mockQuerier{result: &haiku.QueryResult{Text: "2025-01-01"}}
	setCurrentService(NewService(mq))
	defer setCurrentService(nil)

	result, err := Parse(context.Background(), "jan 1st", "date")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}
	if result.Value != "2025-01-01" {
		t.Errorf("value = %q, want %q", result.Value, "2025-01-01")
	}
}

func TestParse_PackageLevelUninitialized(t *testing.T) {
	setCurrentService(nil)
	result, err := Parse(context.Background(), "tomorrow", "date")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected failure")
	}
}

func TestLooksLikeISO8601(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"2025-01-01", true},
		{"2025-01-01T12:00:00", true},
		{"2025-01-01T12:00:00Z", true},
		{"2025-01-01T12:00:00+08:00", true},
		{"tomorrow", false},
		{"jan 1st", false},
		{"2025-01-", false},
		{"2025", false},
		{"", false},
		{"abc", false},
	}
	for _, tt := range tests {
		got := looksLikeISO8601(tt.input)
		if got != tt.want {
			t.Errorf("looksLikeISO8601(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}
