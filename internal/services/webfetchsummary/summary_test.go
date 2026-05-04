package webfetchsummary

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

func TestSummarize_Success(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_WEB_FETCH_SUMMARY", "1")
	mq := &mockQuerier{result: &haiku.QueryResult{Text: "This page describes Go programming."}}
	svc := NewService(mq)

	result, err := svc.Summarize(context.Background(), "Go is a programming language...", "What is this about?", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "This page describes Go programming." {
		t.Errorf("result = %q, want %q", result, "This page describes Go programming.")
	}
	if len(mq.calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(mq.calls))
	}
	if mq.calls[0].QuerySource != querySource {
		t.Errorf("query_source = %q, want %q", mq.calls[0].QuerySource, querySource)
	}
	// Verify the prompt contains the markdown content and the user prompt
	prompt := mq.calls[0].UserPrompt
	if !strings.Contains(prompt, "Go is a programming language") {
		t.Error("expected prompt to contain the markdown content")
	}
	if !strings.Contains(prompt, "What is this about?") {
		t.Error("expected prompt to contain the user prompt")
	}
	// Verify system prompt is empty (matching TS)
	if mq.calls[0].SystemPrompt != "" {
		t.Errorf("system_prompt = %q, want empty", mq.calls[0].SystemPrompt)
	}
}

func TestSummarize_FlagDisabled(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_WEB_FETCH_SUMMARY", "0")
	mq := &mockQuerier{}
	svc := NewService(mq)

	result, err := svc.Summarize(context.Background(), "content", "prompt", false)
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

func TestSummarize_EmptyContent(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_WEB_FETCH_SUMMARY", "1")
	mq := &mockQuerier{}
	svc := NewService(mq)

	result, err := svc.Summarize(context.Background(), "   ", "prompt", false)
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

func TestSummarize_NilService(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_WEB_FETCH_SUMMARY", "1")
	var svc *Service

	result, err := svc.Summarize(context.Background(), "content", "prompt", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("result = %q, want empty", result)
	}
}

func TestSummarize_QuerierError(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_WEB_FETCH_SUMMARY", "1")
	mq := &mockQuerier{err: errors.New("haiku overloaded")}
	svc := NewService(mq)

	result, err := svc.Summarize(context.Background(), "content", "prompt", false)
	if err != ErrHaikuCallFailed {
		t.Fatalf("expected ErrHaikuCallFailed, got %v", err)
	}
	if result != "" {
		t.Errorf("result = %q, want empty", result)
	}
}

func TestSummarize_QuerierReturnsNilResult(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_WEB_FETCH_SUMMARY", "1")
	mq := &mockQuerier{result: nil}
	svc := NewService(mq)

	result, err := svc.Summarize(context.Background(), "content", "prompt", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "No response from model" {
		t.Errorf("result = %q, want %q", result, "No response from model")
	}
}

func TestSummarize_QuerierReturnsEmptyText(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_WEB_FETCH_SUMMARY", "1")
	mq := &mockQuerier{result: &haiku.QueryResult{Text: "   "}}
	svc := NewService(mq)

	result, err := svc.Summarize(context.Background(), "content", "prompt", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "No response from model" {
		t.Errorf("result = %q, want %q", result, "No response from model")
	}
}

func TestSummarize_ContentTruncation(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_WEB_FETCH_SUMMARY", "1")
	longContent := strings.Repeat("a", MaxContentLength+1000)
	mq := &mockQuerier{result: &haiku.QueryResult{Text: "summary"}}
	svc := NewService(mq)

	result, err := svc.Summarize(context.Background(), longContent, "what?", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "summary" {
		t.Errorf("result = %q, want %q", result, "summary")
	}
	// Check that the prompt contains the truncation marker
	prompt := mq.calls[0].UserPrompt
	if !strings.Contains(prompt, "[Content truncated due to length...]") {
		t.Error("expected truncated content to contain truncation marker")
	}
}

func TestSummarize_PreapprovedDomainPrompt(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_WEB_FETCH_SUMMARY", "1")
	mq := &mockQuerier{result: &haiku.QueryResult{Text: "summary"}}
	svc := NewService(mq)

	_, err := svc.Summarize(context.Background(), "content", "what?", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	prompt := mq.calls[0].UserPrompt
	if !strings.Contains(prompt, "Include relevant details, code examples") {
		t.Error("expected preapproved domain prompt to allow code examples")
	}
	if strings.Contains(prompt, "125-character maximum") {
		t.Error("preapproved domain prompt should not include 125-char limit")
	}
}

func TestSummarize_NonPreapprovedDomainPrompt(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_WEB_FETCH_SUMMARY", "1")
	mq := &mockQuerier{result: &haiku.QueryResult{Text: "summary"}}
	svc := NewService(mq)

	_, err := svc.Summarize(context.Background(), "content", "what?", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	prompt := mq.calls[0].UserPrompt
	if !strings.Contains(prompt, "125-character maximum") {
		t.Error("expected non-preapproved domain prompt to include 125-char limit")
	}
}

func TestSummarize_PackageLevel(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_WEB_FETCH_SUMMARY", "1")
	mq := &mockQuerier{result: &haiku.QueryResult{Text: "pkg-summary"}}
	setCurrentService(NewService(mq))
	defer setCurrentService(nil)

	result, err := Summarize(context.Background(), "content", "what?", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "pkg-summary" {
		t.Errorf("result = %q, want %q", result, "pkg-summary")
	}
}

func TestSummarize_PackageLevelUninitialized(t *testing.T) {
	setCurrentService(nil)
	result, err := Summarize(context.Background(), "content", "what?", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("result = %q, want empty", result)
	}
}
