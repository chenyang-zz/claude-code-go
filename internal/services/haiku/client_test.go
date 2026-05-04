package haiku

import (
	"context"
	"errors"
	"net"
	"os"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
	"github.com/sheepzhao/claude-code-go/internal/core/model"
	"github.com/sheepzhao/claude-code-go/internal/platform/api/anthropic"
)

// mockClient implements model.Client for tests.
type mockClient struct {
	stream model.Stream
	err    error
	lastReq model.Request
}

func (m *mockClient) Stream(ctx context.Context, req model.Request) (model.Stream, error) {
	m.lastReq = req
	if m.err != nil {
		return nil, m.err
	}
	return m.stream, nil
}

// streamFromEvents builds a closed channel from a slice of events.
func streamFromEvents(events []model.Event) model.Stream {
	ch := make(chan model.Event, len(events))
	for _, e := range events {
		ch <- e
	}
	close(ch)
	return ch
}

func TestQuery_Success(t *testing.T) {
	events := []model.Event{
		{Type: model.EventTypeTextDelta, Text: "Hello "},
		{Type: model.EventTypeTextDelta, Text: "world"},
		{Type: model.EventTypeDone, StopReason: model.StopReasonEndTurn, Usage: &model.Usage{InputTokens: 10, OutputTokens: 2}},
	}
	mc := &mockClient{stream: streamFromEvents(events)}
	svc := NewService(mc)

	ctx := context.Background()
	result, err := svc.Query(ctx, QueryParams{
		SystemPrompt: "sys",
		UserPrompt:   "user",
		QuerySource:  "test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Text != "Hello world" {
		t.Errorf("text = %q, want %q", result.Text, "Hello world")
	}
	if result.StopReason != string(model.StopReasonEndTurn) {
		t.Errorf("stop_reason = %q, want %q", result.StopReason, model.StopReasonEndTurn)
	}
	if result.Usage.InputTokens != 10 || result.Usage.OutputTokens != 2 {
		t.Errorf("usage = %+v", result.Usage)
	}
	if mc.lastReq.Model != DefaultHaikuModel {
		t.Errorf("model = %q, want default %q", mc.lastReq.Model, DefaultHaikuModel)
	}
	if len(mc.lastReq.Messages) != 1 {
		t.Fatalf("messages len = %d, want 1", len(mc.lastReq.Messages))
	}
	msg := mc.lastReq.Messages[0]
	if msg.Role != message.RoleUser {
		t.Errorf("role = %q, want %q", msg.Role, message.RoleUser)
	}
}

func TestQuery_CustomModelAndMaxTokens(t *testing.T) {
	events := []model.Event{
		{Type: model.EventTypeDone, StopReason: model.StopReasonEndTurn},
	}
	mc := &mockClient{stream: streamFromEvents(events)}
	svc := NewService(mc)

	ctx := context.Background()
	_, _ = svc.Query(ctx, QueryParams{
		Model:           "custom-model",
		MaxOutputTokens: 512,
	})
	if mc.lastReq.Model != "custom-model" {
		t.Errorf("model = %q, want %q", mc.lastReq.Model, "custom-model")
	}
	if mc.lastReq.MaxOutputTokens != 512 {
		t.Errorf("max_tokens = %d, want 512", mc.lastReq.MaxOutputTokens)
	}
}

func TestQuery_PromptCaching(t *testing.T) {
	events := []model.Event{
		{Type: model.EventTypeDone, StopReason: model.StopReasonEndTurn},
	}
	mc := &mockClient{stream: streamFromEvents(events)}
	svc := NewService(mc)

	ctx := context.Background()
	_, _ = svc.Query(ctx, QueryParams{EnablePromptCaching: true})
	if !mc.lastReq.EnablePromptCaching {
		t.Error("EnablePromptCaching not set")
	}
}

func TestQuery_NilService(t *testing.T) {
	var svc *Service
	_, err := svc.Query(context.Background(), QueryParams{})
	if !errors.Is(err, ErrClientUnavailable) {
		t.Errorf("err = %v, want ErrClientUnavailable", err)
	}
}

func TestQuery_NilClient(t *testing.T) {
	svc := NewService(nil)
	if svc != nil {
		t.Fatal("expected nil service when client is nil")
	}
}

func TestQuery_FlagDisabled(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_HAIKU", "0")
	mc := &mockClient{stream: streamFromEvents(nil)}
	svc := NewService(mc)

	_, err := svc.Query(context.Background(), QueryParams{})
	if !errors.Is(err, ErrHaikuDisabled) {
		t.Errorf("err = %v, want ErrHaikuDisabled", err)
	}
}

func TestQuery_ContextCancelled(t *testing.T) {
	mc := &mockClient{stream: streamFromEvents(nil)}
	svc := NewService(mc)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := svc.Query(ctx, QueryParams{})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

func TestQuery_StreamError(t *testing.T) {
	mc := &mockClient{err: errors.New("dial failed")}
	svc := NewService(mc)

	_, err := svc.Query(context.Background(), QueryParams{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestQuery_ModelErrorEvent(t *testing.T) {
	events := []model.Event{
		{Type: model.EventTypeError, Error: "overloaded"},
	}
	mc := &mockClient{stream: streamFromEvents(events)}
	svc := NewService(mc)

	_, err := svc.Query(context.Background(), QueryParams{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestQuery_PackageLevelQuery(t *testing.T) {
	// When no singleton is set, package-level Query should return ErrClientUnavailable.
	setCurrentService(nil)
	_, err := Query(context.Background(), QueryParams{})
	if !errors.Is(err, ErrClientUnavailable) {
		t.Errorf("err = %v, want ErrClientUnavailable", err)
	}
}

func TestQuery_PackageLevelQueryWithSingleton(t *testing.T) {
	events := []model.Event{
		{Type: model.EventTypeTextDelta, Text: "ok"},
		{Type: model.EventTypeDone, StopReason: model.StopReasonEndTurn},
	}
	mc := &mockClient{stream: streamFromEvents(events)}
	svc := NewService(mc)
	setCurrentService(svc)
	defer setCurrentService(nil)

	result, err := Query(context.Background(), QueryParams{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Text != "ok" {
		t.Errorf("text = %q, want %q", result.Text, "ok")
	}
}

func TestIsRateLimit(t *testing.T) {
	apiErr := &anthropic.APIError{Status: 429, Type: anthropic.ErrorTypeRateLimit}
	if !IsRateLimit(apiErr) {
		t.Error("expected rate limit")
	}
	if IsRateLimit(errors.New("random")) {
		t.Error("expected false for random error")
	}
	if IsRateLimit(nil) {
		t.Error("expected false for nil")
	}
}

func TestIsNetwork(t *testing.T) {
	if !IsNetwork(context.DeadlineExceeded) {
		t.Error("expected DeadlineExceeded to be network")
	}
	if !IsNetwork(context.Canceled) {
		t.Error("expected Canceled to be network")
	}
	var netErr net.Error = &net.DNSError{Err: "no such host", Name: "x"}
	if !IsNetwork(netErr) {
		t.Error("expected net.Error to be network")
	}
	if IsNetwork(&anthropic.APIError{Status: 500, Type: anthropic.ErrorTypeAPIError}) {
		t.Error("expected APIError not to be network")
	}
	if IsNetwork(nil) {
		t.Error("expected false for nil")
	}
}

func TestIsAPIError(t *testing.T) {
	if !IsAPIError(&anthropic.APIError{Status: 500, Type: anthropic.ErrorTypeAPIError}) {
		t.Error("expected APIError")
	}
	if IsAPIError(errors.New("random")) {
		t.Error("expected false for random error")
	}
	if IsAPIError(nil) {
		t.Error("expected false for nil")
	}
}

func TestIsHaikuEnabled_Default(t *testing.T) {
	os.Unsetenv("CLAUDE_FEATURE_HAIKU")
	if !IsHaikuEnabled() {
		t.Error("expected default enabled")
	}
}

func TestIsHaikuEnabled_Disabled(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_HAIKU", "0")
	if IsHaikuEnabled() {
		t.Error("expected disabled")
	}
}

func TestIsHaikuEnabled_FalseString(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_HAIKU", "false")
	if IsHaikuEnabled() {
		t.Error("expected disabled")
	}
}

func TestInitHaiku_Disabled(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_HAIKU", "0")
	setCurrentService(nil)
	result := InitHaiku(InitOptions{Client: &mockClient{}})
	if result != nil {
		t.Error("expected nil when disabled")
	}
	if IsInitialized() {
		t.Error("expected not initialized")
	}
}

func TestInitHaiku_NilClient(t *testing.T) {
	setCurrentService(nil)
	result := InitHaiku(InitOptions{Client: nil})
	if result != nil {
		t.Error("expected nil when client nil")
	}
}

func TestInitHaiku_Success(t *testing.T) {
	setCurrentService(nil)
	mc := &mockClient{}
	result := InitHaiku(InitOptions{Client: mc})
	if result == nil {
		t.Fatal("expected service")
	}
	if !IsInitialized() {
		t.Error("expected initialized")
	}
	if CurrentService() != result {
		t.Error("CurrentService mismatch")
	}
}
