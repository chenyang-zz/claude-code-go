package awaysummary

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
	"github.com/sheepzhao/claude-code-go/internal/core/model"
)

func TestBuildPrompt_withoutMemory(t *testing.T) {
	result := buildPrompt("")
	if !strings.Contains(result, "The user stepped away and is coming back") {
		t.Errorf("expected prompt to contain the away message, got: %s", result)
	}
	if !strings.Contains(result, "1-3 short sentences") {
		t.Errorf("expected prompt to mention sentence count, got: %s", result)
	}
	if strings.Contains(result, "Session memory") {
		t.Error("expected prompt NOT to contain memory section when memory is empty")
	}
}

func TestBuildPrompt_withMemory(t *testing.T) {
	memory := "User is building a CLI tool in Go."
	result := buildPrompt(memory)
	if !strings.Contains(result, "Session memory (broader context):") {
		t.Error("expected prompt to contain memory section header")
	}
	if !strings.Contains(result, memory) {
		t.Errorf("expected prompt to contain memory content, got: %s", result)
	}
	if !strings.Contains(result, "The user stepped away") {
		t.Error("expected prompt to contain away message after memory")
	}
}

func TestSystem_ShouldGenerate_beforeIdleThreshold(t *testing.T) {
	cfg := Config{
		IdleThreshold: 5 * time.Minute,
		Model:         "test-model",
		MaxMessages:   30,
	}
	sys := NewSystem(nil, "", cfg)
	sys.RecordActivity()
	if sys.ShouldGenerate() {
		t.Error("expected ShouldGenerate to be false immediately after RecordActivity")
	}
}

func TestSystem_ShouldGenerate_afterIdleThreshold(t *testing.T) {
	cfg := Config{
		IdleThreshold: 1 * time.Millisecond,
		Model:         "test-model",
		MaxMessages:   30,
	}
	sys := NewSystem(nil, "", cfg)
	time.Sleep(5 * time.Millisecond)
	if !sys.ShouldGenerate() {
		t.Error("expected ShouldGenerate to be true after idle threshold exceeded")
	}
}

func TestSystem_RecordActivity_resetsIdleTimer(t *testing.T) {
	cfg := Config{
		IdleThreshold: 1 * time.Millisecond,
		Model:         "test-model",
		MaxMessages:   30,
	}
	sys := NewSystem(nil, "", cfg)
	time.Sleep(5 * time.Millisecond)
	if !sys.ShouldGenerate() {
		t.Error("expected ShouldGenerate true before RecordActivity")
	}
	sys.RecordActivity()
	if sys.ShouldGenerate() {
		t.Error("expected ShouldGenerate false after RecordActivity resets timer")
	}
}

func TestSystem_Generate_emptyMessages(t *testing.T) {
	cfg := DefaultConfig()
	sys := NewSystem(nil, "", cfg)
	text, err := sys.Generate(context.Background(), nil)
	if err != nil {
		t.Errorf("unexpected error for empty messages: %v", err)
	}
	if text != "" {
		t.Errorf("expected empty text for empty messages, got: %s", text)
	}
}

func TestSystem_Generate_nilClient(t *testing.T) {
	cfg := DefaultConfig()
	sys := NewSystem(nil, "", cfg)
	msgs := []message.Message{
		{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("hello")}},
		{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("hi")}},
	}
	text, err := sys.Generate(context.Background(), msgs)
	if err != nil {
		t.Errorf("unexpected error with nil client: %v", err)
	}
	if text != "" {
		t.Errorf("expected empty text when client is nil, got: %s", text)
	}
}

// mockClient implements model.Client for testing.
type mockClient struct {
	events []model.Event
}

func (m *mockClient) Stream(ctx context.Context, req model.Request) (model.Stream, error) {
	ch := make(chan model.Event, len(m.events))
	for _, e := range m.events {
		ch <- e
	}
	close(ch)
	return ch, nil
}

func TestSystem_Generate_withMockClient(t *testing.T) {
	events := []model.Event{
		{Type: model.EventTypeTextDelta, Text: "Building a "},
		{Type: model.EventTypeTextDelta, Text: "CLI tool in Go."},
		{Type: model.EventTypeDone},
	}
	client := &mockClient{events: events}
	cfg := DefaultConfig()
	sys := NewSystem(client, "", cfg)

	msgs := []message.Message{
		{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("I am building a CLI tool")}},
		{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("Let me help with that")}},
	}

	text, err := sys.Generate(context.Background(), msgs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if text != "Building a CLI tool in Go." {
		t.Errorf("expected concatenated text, got: %s", text)
	}
}

func TestSystem_Generate_withMockClient_error(t *testing.T) {
	events := []model.Event{
		{Type: model.EventTypeError, Error: "rate limit exceeded"},
	}
	client := &mockClient{events: events}
	cfg := DefaultConfig()
	sys := NewSystem(client, "", cfg)

	msgs := []message.Message{
		{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("hello")}},
	}

	_, err := sys.Generate(context.Background(), msgs)
	if err == nil {
		t.Fatal("expected error from model error event")
	}
	if !strings.Contains(err.Error(), "rate limit exceeded") {
		t.Errorf("expected error to contain rate limit message, got: %v", err)
	}
}

func TestSystem_Generate_contextCanceled(t *testing.T) {
	// Create a client that blocks until context is canceled.
	client := &blockingClient{}
	cfg := DefaultConfig()
	sys := NewSystem(client, "", cfg)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	msgs := []message.Message{
		{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("hello")}},
	}

	_, err := sys.Generate(ctx, msgs)
	if err == nil {
		t.Fatal("expected error from canceled context")
	}
}

type blockingClient struct{}

func (b *blockingClient) Stream(ctx context.Context, req model.Request) (model.Stream, error) {
	ch := make(chan model.Event)
	go func() {
		<-ctx.Done()
		close(ch)
	}()
	return ch, nil
}

func TestSystem_Generate_truncatesToMaxMessages(t *testing.T) {
	events := []model.Event{
		{Type: model.EventTypeTextDelta, Text: "summary"},
		{Type: model.EventTypeDone},
	}
	client := &mockClient{events: events}
	cfg := Config{
		IdleThreshold: 5 * time.Minute,
		Model:         "test-model",
		MaxMessages:   2,
	}
	sys := NewSystem(client, "", cfg)

	// 5 messages, only last 2 should be used
	msgs := []message.Message{
		{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("msg1")}},
		{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("r1")}},
		{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("msg2")}},
		{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("r2")}},
		{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("msg3")}},
	}

	text, err := sys.Generate(context.Background(), msgs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if text != "summary" {
		t.Errorf("expected summary text, got: %s", text)
	}
}

func TestReadMemoryContent_emptyDir(t *testing.T) {
	result := readMemoryContent("")
	if result != "" {
		t.Errorf("expected empty string for empty dir, got: %s", result)
	}
}

func TestReadMemoryContent_nonexistentDir(t *testing.T) {
	result := readMemoryContent("/nonexistent/path/memory")
	if result != "" {
		t.Errorf("expected empty string for nonexistent dir, got: %s", result)
	}
}

func TestReadMemoryContent_validDir(t *testing.T) {
	dir := t.TempDir()
	indexPath := filepath.Join(dir, memoryIndexName)
	content := "- [User Role](user_role.md) — software engineer\n- [Project Plan](plan.md) — Q3 migration"
	if err := os.WriteFile(indexPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test MEMORY.md: %v", err)
	}

	result := readMemoryContent(dir)
	if result != content {
		t.Errorf("expected memory content:\n%s\ngot:\n%s", content, result)
	}
}

func TestCheckAndGenerate_nilSystem(t *testing.T) {
	// Reset currentSystem to nil for this test.
	oldSys := currentSystem
	currentSystem = nil
	defer func() { currentSystem = oldSys }()

	msg := CheckAndGenerate(context.Background(), nil)
	if msg != nil {
		t.Error("expected nil message when system is not initialized")
	}
}

func TestCheckAndGenerate_notIdle(t *testing.T) {
	cfg := Config{
		IdleThreshold: 5 * time.Minute,
		Model:         "test-model",
		MaxMessages:   30,
	}
	oldSys := currentSystem
	currentSystem = NewSystem(nil, "", cfg)
	currentSystem.RecordActivity()
	defer func() { currentSystem = oldSys }()

	msg := CheckAndGenerate(context.Background(), nil)
	if msg != nil {
		t.Error("expected nil message when not idle long enough")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.IdleThreshold != DefaultIdleThreshold {
		t.Errorf("expected IdleThreshold %v, got %v", DefaultIdleThreshold, cfg.IdleThreshold)
	}
	if cfg.Model != DefaultModel {
		t.Errorf("expected Model %s, got %s", DefaultModel, cfg.Model)
	}
	if cfg.MaxMessages != DefaultMaxMessages {
		t.Errorf("expected MaxMessages %d, got %d", DefaultMaxMessages, cfg.MaxMessages)
	}
}
