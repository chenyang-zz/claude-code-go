package engine

import (
	"context"
	"errors"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
)

// TestPostTurnHook_RegisterAndFire verifies that a registered hook is called
// when firePostTurnHooks executes.
func TestPostTurnHook_RegisterAndFire(t *testing.T) {
	ClearPostTurnHooks()
	called := false
	RegisterPostTurnHook(func(ctx context.Context, messages []message.Message, workingDir string) error {
		called = true
		return nil
	})

	e := &Runtime{}
	msgs := []message.Message{}
	e.firePostTurnHooks(context.Background(), "/test/cwd", msgs)

	if !called {
		t.Error("post-turn hook was not called")
	}
}

// TestPostTurnHook_Clear verifies that ClearPostTurnHooks removes all hooks
// so none are executed.
func TestPostTurnHook_Clear(t *testing.T) {
	ClearPostTurnHooks()
	RegisterPostTurnHook(func(ctx context.Context, messages []message.Message, workingDir string) error {
		t.Error("hook should not have been called")
		return nil
	})
	ClearPostTurnHooks()

	e := &Runtime{}
	msgs := []message.Message{}
	e.firePostTurnHooks(context.Background(), "/test/cwd", msgs)
}

// TestPostTurnHook_MultipleHooks verifies that all registered hooks are called.
func TestPostTurnHook_MultipleHooks(t *testing.T) {
	ClearPostTurnHooks()
	count := 0
	hook := func(ctx context.Context, messages []message.Message, workingDir string) error {
		count++
		return nil
	}
	RegisterPostTurnHook(hook)
	RegisterPostTurnHook(hook)

	e := &Runtime{}
	msgs := []message.Message{}
	e.firePostTurnHooks(context.Background(), "/test/cwd", msgs)

	if count != 2 {
		t.Errorf("expected 2 hooks to be called, got %d", count)
	}
}

// TestPostTurnHook_HookError verifies that a hook returning an error does not
// cause a panic and does not prevent subsequent hooks from executing.
func TestPostTurnHook_HookError(t *testing.T) {
	ClearPostTurnHooks()
	secondCalled := false
	RegisterPostTurnHook(func(ctx context.Context, messages []message.Message, workingDir string) error {
		return errors.New("hook failure")
	})
	RegisterPostTurnHook(func(ctx context.Context, messages []message.Message, workingDir string) error {
		secondCalled = true
		return nil
	})

	e := &Runtime{}
	msgs := []message.Message{}
	e.firePostTurnHooks(context.Background(), "/test/cwd", msgs)

	if !secondCalled {
		t.Error("second hook was not called after first hook returned an error")
	}
}

// TestHasPendingToolCalls_NoCalls verifies that hasPendingToolCalls returns
// false when the latest assistant message contains no tool_use blocks.
func TestHasPendingToolCalls_NoCalls(t *testing.T) {
	msgs := []message.Message{
		{
			Role: message.RoleAssistant,
			Content: []message.ContentPart{
				{Type: "text", Text: "Hello"},
			},
		},
	}
	if hasPendingToolCalls(msgs) {
		t.Error("expected no pending tool calls")
	}
}

// TestHasPendingToolCalls_WithCalls verifies that hasPendingToolCalls returns
// true when the latest assistant message contains a tool_use block.
func TestHasPendingToolCalls_WithCalls(t *testing.T) {
	msgs := []message.Message{
		{
			Role: message.RoleAssistant,
			Content: []message.ContentPart{
				{Type: "tool_use", ToolUseID: "tool_1", ToolName: "file_edit"},
			},
		},
	}
	if !hasPendingToolCalls(msgs) {
		t.Error("expected pending tool calls")
	}
}

// TestHasPendingToolCalls_NoAssistantMessage verifies that hasPendingToolCalls
// returns false when there are no assistant messages in the history.
func TestHasPendingToolCalls_NoAssistantMessage(t *testing.T) {
	msgs := []message.Message{
		{
			Role: message.RoleUser,
			Content: []message.ContentPart{
				{Type: "text", Text: "hello"},
			},
		},
	}
	if hasPendingToolCalls(msgs) {
		t.Error("expected no pending tool calls when no assistant message exists")
	}
}

// TestHasPendingToolCalls_EmptyMessages verifies that hasPendingToolCalls
// returns false for an empty message list.
func TestHasPendingToolCalls_EmptyMessages(t *testing.T) {
	msgs := []message.Message{}
	if hasPendingToolCalls(msgs) {
		t.Error("expected no pending tool calls for empty messages")
	}
}
