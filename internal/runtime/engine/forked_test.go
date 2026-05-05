package engine

import (
	"context"
	"errors"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
	"github.com/sheepzhao/claude-code-go/internal/core/model"
)

// --- Mock helpers ---

// forkMockClient implements model.Client for forked agent tests.
type forkMockClient struct {
	stream     chan model.Event
	err        error
	streamFunc func(ctx context.Context, req model.Request) (model.Stream, error)
}

func (m *forkMockClient) Stream(ctx context.Context, req model.Request) (model.Stream, error) {
	if m.streamFunc != nil {
		return m.streamFunc(ctx, req)
	}
	return m.stream, m.err
}

// sendEvents writes the given events to the channel and closes it.
func sendEvents(ch chan<- model.Event, events []model.Event) {
	go func() {
		for _, e := range events {
			ch <- e
		}
		close(ch)
	}()
}

// --- Type tests ---

func TestCacheSafeParams_Creation(t *testing.T) {
	rt := &Runtime{DefaultModel: "test-model"}
	msgs := []message.Message{
		{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("hello")}},
	}

	params := CacheSafeParams{
		SystemPrompt: "system prompt",
		SystemContext: map[string]string{"key": "value"},
		Messages:      msgs,
		Runtime:       rt,
	}

	if params.SystemPrompt != "system prompt" {
		t.Errorf("SystemPrompt = %q, want %q", params.SystemPrompt, "system prompt")
	}
	if params.Runtime != rt {
		t.Error("Runtime should point to the provided runtime")
	}
	if len(params.Messages) != 1 {
		t.Errorf("Messages length = %d, want 1", len(params.Messages))
	}
}

func TestForkedAgentParams_Creation(t *testing.T) {
	params := ForkedAgentParams{
		PromptMessages: []message.Message{
			{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("prompt")}},
		},
		CanUseTool:      AllowAllTools,
		ForkLabel:       "test_fork",
		MaxOutputTokens: 1024,
		MaxTurns:        3,
		SkipTranscript:  true,
	}

	if params.ForkLabel != "test_fork" {
		t.Errorf("ForkLabel = %q, want %q", params.ForkLabel, "test_fork")
	}
	if params.MaxTurns != 3 {
		t.Errorf("MaxTurns = %d, want 3", params.MaxTurns)
	}
	if !params.SkipTranscript {
		t.Error("SkipTranscript should be true")
	}
}

func TestForkedAgentResult_Creation(t *testing.T) {
	result := ForkedAgentResult{
		Messages: []message.Message{
			{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("response")}},
		},
		TotalUsage: model.Usage{
			InputTokens:  100,
			OutputTokens: 50,
		},
	}

	if len(result.Messages) != 1 {
		t.Errorf("Messages length = %d, want 1", len(result.Messages))
	}
	if result.TotalUsage.InputTokens != 100 {
		t.Errorf("InputTokens = %d, want 100", result.TotalUsage.InputTokens)
	}
}

func TestSubagentContextOverrides_Creation(t *testing.T) {
	overrides := SubagentContextOverrides{
		Model:       "custom-model",
		ToolCatalog: []model.ToolDefinition{{Name: "tool1"}},
	}

	if overrides.Model != "custom-model" {
		t.Errorf("Model = %q, want %q", overrides.Model, "custom-model")
	}
	if len(overrides.ToolCatalog) != 1 {
		t.Errorf("ToolCatalog length = %d, want 1", len(overrides.ToolCatalog))
	}
}

// --- CanUseToolFn tests ---

func TestAllowAllTools(t *testing.T) {
	if !AllowAllTools("any_tool") {
		t.Error("AllowAllTools should return true for any tool")
	}
}

func TestDenyAllTools(t *testing.T) {
	if DenyAllTools("any_tool") {
		t.Error("DenyAllTools should return false for any tool")
	}
}

// --- Global state tests ---

func TestSaveAndGetLastCacheSafeParams(t *testing.T) {
	// Clear any existing state.
	SaveCacheSafeParams(nil)
	if GetLastCacheSafeParams() != nil {
		t.Error("GetLastCacheSafeParams should return nil after saving nil")
	}

	params := &CacheSafeParams{
		SystemPrompt: "test",
		Runtime:      &Runtime{DefaultModel: "model"},
	}
	SaveCacheSafeParams(params)

	got := GetLastCacheSafeParams()
	if got == nil {
		t.Fatal("GetLastCacheSafeParams should not return nil after saving params")
	}
	if got.SystemPrompt != "test" {
		t.Errorf("SystemPrompt = %q, want %q", got.SystemPrompt, "test")
	}

	// Clean up.
	SaveCacheSafeParams(nil)
}

// --- CreateSubagentContext tests ---

func TestCreateSubagentContext_NilOverrides(t *testing.T) {
	parent := &Runtime{
		DefaultModel:       "parent-model",
		AutoCompact:        true,
		MaxToolIterations:  5,
		EnablePromptCaching: true,
	}

	forked := CreateSubagentContext(parent, nil)

	if forked.DefaultModel != "parent-model" {
		t.Errorf("DefaultModel = %q, want %q", forked.DefaultModel, "parent-model")
	}
	if forked.AutoCompact {
		t.Error("AutoCompact should be false for forked agents")
	}
	if forked.MaxToolIterations != 5 {
		t.Errorf("MaxToolIterations = %d, want 5", forked.MaxToolIterations)
	}
	if !forked.EnablePromptCaching {
		t.Error("EnablePromptCaching should be inherited from parent")
	}
}

func TestCreateSubagentContext_WithModelOverride(t *testing.T) {
	parent := &Runtime{
		DefaultModel: "parent-model",
		ToolCatalog:  []model.ToolDefinition{{Name: "tool1"}},
	}

	overrides := &SubagentContextOverrides{
		Model: "forked-model",
	}
	forked := CreateSubagentContext(parent, overrides)

	if forked.DefaultModel != "forked-model" {
		t.Errorf("DefaultModel = %q, want %q", forked.DefaultModel, "forked-model")
	}
	// Original tool catalog should still be there.
	if len(forked.ToolCatalog) != 1 {
		t.Errorf("ToolCatalog length = %d, want 1", len(forked.ToolCatalog))
	}
}

func TestCreateSubagentContext_WithToolCatalogOverride(t *testing.T) {
	parent := &Runtime{
		DefaultModel: "parent-model",
		ToolCatalog:  []model.ToolDefinition{{Name: "tool1"}, {Name: "tool2"}},
	}

	overrides := &SubagentContextOverrides{
		ToolCatalog: []model.ToolDefinition{{Name: "custom_tool"}},
	}
	forked := CreateSubagentContext(parent, overrides)

	if len(forked.ToolCatalog) != 1 {
		t.Errorf("ToolCatalog length = %d, want 1", len(forked.ToolCatalog))
	}
	if forked.ToolCatalog[0].Name != "custom_tool" {
		t.Errorf("ToolCatalog[0].Name = %q, want %q", forked.ToolCatalog[0].Name, "custom_tool")
	}
}

func TestCreateSubagentContext_Isolation(t *testing.T) {
	parent := &Runtime{
		DefaultModel: "parent-model",
		ToolCatalog:  []model.ToolDefinition{{Name: "tool1"}},
	}

	forked := CreateSubagentContext(parent, nil)

	// Modify forked's tool catalog.
	forked.ToolCatalog[0].Name = "modified"

	// Parent should not be affected.
	if parent.ToolCatalog[0].Name != "tool1" {
		t.Error("Modifying forked tool catalog should not affect parent")
	}
}

func TestCreateSubagentContext_DisablesAutoCompact(t *testing.T) {
	parent := &Runtime{
		DefaultModel: "model",
		AutoCompact:  true,
	}

	forked := CreateSubagentContext(parent, nil)
	if forked.AutoCompact {
		t.Error("Forked agent should have AutoCompact disabled")
	}
}

// --- RunForked tests ---

func TestRunForked_NilRuntime(t *testing.T) {
	params := ForkedAgentParams{
		ForkLabel: "test",
	}
	_, err := RunForked(context.Background(), params)
	if err == nil {
		t.Error("RunForked should return error when runtime is nil")
	}
}

func TestRunForked_NilClient(t *testing.T) {
	params := ForkedAgentParams{
		ForkLabel: "test",
		CacheSafeParams: CacheSafeParams{
			Runtime: &Runtime{},
		},
	}
	_, err := RunForked(context.Background(), params)
	if err == nil {
		t.Error("RunForked should return error when client is nil")
	}
}

func TestRunForked_StreamError(t *testing.T) {
	client := &forkMockClient{err: errors.New("stream failed")}
	rt := &Runtime{
		Client:       client,
		DefaultModel: "test-model",
	}

	params := ForkedAgentParams{
		ForkLabel: "test",
		CacheSafeParams: CacheSafeParams{
			Runtime:      rt,
			SystemPrompt: "system",
		},
		PromptMessages: []message.Message{
			{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("hello")}},
		},
	}
	_, err := RunForked(context.Background(), params)
	if err == nil {
		t.Error("RunForked should return error when stream fails")
	}
}

func TestRunForked_SimpleTextResponse(t *testing.T) {
	stream := make(chan model.Event)
	client := &forkMockClient{stream: stream}
	rt := &Runtime{
		Client:       client,
		DefaultModel: "test-model",
	}

	go func() {
		stream <- model.Event{Type: model.EventTypeTextDelta, Text: "Hello"}
		stream <- model.Event{Type: model.EventTypeDone, StopReason: model.StopReasonEndTurn, Usage: &model.Usage{InputTokens: 10, OutputTokens: 5}}
		close(stream)
	}()

	params := ForkedAgentParams{
		ForkLabel: "test",
		CacheSafeParams: CacheSafeParams{
			Runtime: rt,
		},
		PromptMessages: []message.Message{
			{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("hi")}},
		},
		SkipTranscript: true,
	}

	result, err := RunForked(context.Background(), params)
	if err != nil {
		t.Fatalf("RunForked error: %v", err)
	}
	if len(result.Messages) != 1 {
		t.Fatalf("Messages length = %d, want 1", len(result.Messages))
	}
	if result.Messages[0].Role != message.RoleAssistant {
		t.Errorf("Message role = %q, want %q", result.Messages[0].Role, message.RoleAssistant)
	}
	if result.TotalUsage.InputTokens != 10 {
		t.Errorf("InputTokens = %d, want 10", result.TotalUsage.InputTokens)
	}
	if result.TotalUsage.OutputTokens != 5 {
		t.Errorf("OutputTokens = %d, want 5", result.TotalUsage.OutputTokens)
	}
}

func TestRunForked_ToolUseDenyAll(t *testing.T) {
	stream := make(chan model.Event)
	client := &forkMockClient{stream: stream}
	rt := &Runtime{
		Client:       client,
		DefaultModel: "test-model",
	}

	// Model requests a tool, but DenyAllTools denies it, model then finishes.
	go func() {
		stream <- model.Event{Type: model.EventTypeToolUse, ToolUse: &model.ToolUse{ID: "call1", Name: "Bash"}}
		stream <- model.Event{Type: model.EventTypeDone, StopReason: model.StopReasonToolUse, Usage: &model.Usage{InputTokens: 20, OutputTokens: 10}}
		close(stream)
	}()

	// Second call: model responds with text after tool denial.
	stream2 := make(chan model.Event)
	callCount := 0
	client.streamFunc = func(ctx context.Context, req model.Request) (model.Stream, error) {
		callCount++
		if callCount == 1 {
			return stream, nil
		}
		return stream2, nil
	}

	go func() {
		stream2 <- model.Event{Type: model.EventTypeTextDelta, Text: "done"}
		stream2 <- model.Event{Type: model.EventTypeDone, StopReason: model.StopReasonEndTurn, Usage: &model.Usage{InputTokens: 5, OutputTokens: 3}}
		close(stream2)
	}()

	params := ForkedAgentParams{
		ForkLabel:  "test",
		CanUseTool: DenyAllTools,
		CacheSafeParams: CacheSafeParams{
			Runtime: rt,
		},
		PromptMessages: []message.Message{
			{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("run something")}},
		},
		SkipTranscript: true,
	}

	result, err := RunForked(context.Background(), params)
	if err != nil {
		t.Fatalf("RunForked error: %v", err)
	}
	if len(result.Messages) == 0 {
		t.Fatal("Expected at least one message")
	}
}

func TestRunForked_MaxTurnsLimit(t *testing.T) {
	stream := make(chan model.Event)
	client := &forkMockClient{stream: stream}
	rt := &Runtime{
		Client:       client,
		DefaultModel: "test-model",
	}

	go func() {
		stream <- model.Event{Type: model.EventTypeTextDelta, Text: "turn1"}
		stream <- model.Event{Type: model.EventTypeDone, StopReason: model.StopReasonEndTurn, Usage: &model.Usage{InputTokens: 5, OutputTokens: 3}}
		close(stream)
	}()

	params := ForkedAgentParams{
		ForkLabel:  "test",
		MaxTurns:   1,
		CanUseTool: AllowAllTools,
		CacheSafeParams: CacheSafeParams{
			Runtime: rt,
		},
		PromptMessages: []message.Message{
			{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("hello")}},
		},
		SkipTranscript: true,
	}

	result, err := RunForked(context.Background(), params)
	if err != nil {
		t.Fatalf("RunForked error: %v", err)
	}
	if len(result.Messages) == 0 {
		t.Fatal("Expected at least one message")
	}
}

func TestRunForked_UsageAccumulation(t *testing.T) {
	stream := make(chan model.Event)
	client := &forkMockClient{stream: stream}
	rt := &Runtime{
		Client:       client,
		DefaultModel: "test-model",
	}

	go func() {
		stream <- model.Event{Type: model.EventTypeTextDelta, Text: "response"}
		stream <- model.Event{Type: model.EventTypeDone, StopReason: model.StopReasonEndTurn, Usage: &model.Usage{
			InputTokens:              100,
			OutputTokens:             50,
			CacheCreationInputTokens: 25,
			CacheReadInputTokens:     75,
		}}
		close(stream)
	}()

	params := ForkedAgentParams{
		ForkLabel: "test",
		CacheSafeParams: CacheSafeParams{
			Runtime: rt,
		},
		PromptMessages: []message.Message{
			{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("hello")}},
		},
		SkipTranscript: true,
	}

	result, err := RunForked(context.Background(), params)
	if err != nil {
		t.Fatalf("RunForked error: %v", err)
	}
	if result.TotalUsage.InputTokens != 100 {
		t.Errorf("InputTokens = %d, want 100", result.TotalUsage.InputTokens)
	}
	if result.TotalUsage.OutputTokens != 50 {
		t.Errorf("OutputTokens = %d, want 50", result.TotalUsage.OutputTokens)
	}
	if result.TotalUsage.CacheCreationInputTokens != 25 {
		t.Errorf("CacheCreationInputTokens = %d, want 25", result.TotalUsage.CacheCreationInputTokens)
	}
	if result.TotalUsage.CacheReadInputTokens != 75 {
		t.Errorf("CacheReadInputTokens = %d, want 75", result.TotalUsage.CacheReadInputTokens)
	}
}

// --- Helper function tests ---

func TestExtractResultText_WithAssistantMessage(t *testing.T) {
	messages := []message.Message{
		{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("question")}},
		{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("answer")}},
	}

	text := ExtractResultText(messages, "default")
	if text != "answer" {
		t.Errorf("ExtractResultText = %q, want %q", text, "answer")
	}
}

func TestExtractResultText_NoAssistantMessage(t *testing.T) {
	messages := []message.Message{
		{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("question")}},
	}

	text := ExtractResultText(messages, "default")
	if text != "default" {
		t.Errorf("ExtractResultText = %q, want %q", text, "default")
	}
}

func TestExtractResultText_EmptyMessages(t *testing.T) {
	text := ExtractResultText(nil, "fallback")
	if text != "fallback" {
		t.Errorf("ExtractResultText = %q, want %q", text, "fallback")
	}
}

func TestExtractResultText_DefaultTextFallback(t *testing.T) {
	text := ExtractResultText(nil, "")
	if text != "Execution completed" {
		t.Errorf("ExtractResultText = %q, want %q", text, "Execution completed")
	}
}

func TestCreateCacheSafeParams(t *testing.T) {
	rt := &Runtime{DefaultModel: "model"}
	msgs := []message.Message{
		{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("hello")}},
	}

	params := CreateCacheSafeParams(rt, "system", map[string]string{"k": "v"}, msgs)

	if params.SystemPrompt != "system" {
		t.Errorf("SystemPrompt = %q, want %q", params.SystemPrompt, "system")
	}
	if params.Runtime != rt {
		t.Error("Runtime should match")
	}
	if len(params.Messages) != 1 {
		t.Errorf("Messages length = %d, want 1", len(params.Messages))
	}
	// Verify the slice is a copy (different slice header).
	msgs = append(msgs, message.Message{Role: message.RoleAssistant})
	if len(params.Messages) != 1 {
		t.Error("Appending to original should not affect params.Messages")
	}
}

func TestNewAgentID_Unique(t *testing.T) {
	id1 := newAgentID()
	id2 := newAgentID()
	if id1 == id2 {
		t.Errorf("newAgentID should generate unique IDs, got %q twice", id1)
	}
	if len(id1) != 16 {
		t.Errorf("newAgentID length = %d, want 16", len(id1))
	}
}
