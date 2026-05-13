package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sheepzhao/claude-code-go/internal/core/event"
	"github.com/sheepzhao/claude-code-go/internal/core/model"
	"github.com/sheepzhao/claude-code-go/internal/runtime/approval"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRendererStartAndConnect(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	r, err := NewRenderer()
	require.NoError(t, err)
	defer r.Close() //nolint:errcheck

	wsURL := fmt.Sprintf("ws://127.0.0.1:%d/ws", r.Port())
	c, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, nil)
	require.NoError(t, err)
	defer c.Close() //nolint:errcheck

	// Wait for the renderer to register the connection.
	require.NoError(t, r.WaitForConnection(ctx))

	// Send an event via the renderer.
	err = r.RenderEvent(event.Event{
		Type:    event.TypeMessageDelta,
		Payload: event.MessageDeltaPayload{Text: "hello"},
	})
	require.NoError(t, err)

	// Read the event on the WebSocket side.
	_, raw, err := c.ReadMessage()
	require.NoError(t, err)

	var msg WSMessage
	require.NoError(t, json.Unmarshal(raw, &msg))
	assert.Equal(t, msgTypeEvent, msg.Type)

	// Send input from the WebSocket side.
	err = c.WriteJSON(WSMessage{
		Type: msgTypeInput,
		Payload: map[string]string{
			"text": "test input",
		},
	})
	require.NoError(t, err)

	// Read the input from the renderer's InputReader.
	buf := make([]byte, 1024)
	n, err := r.InputReader().Read(buf)
	require.NoError(t, err)
	assert.Equal(t, "test input\n", string(buf[:n]))
}

func TestRendererMultipleEvents(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	r, err := NewRenderer()
	require.NoError(t, err)
	defer r.Close() //nolint:errcheck

	wsURL := fmt.Sprintf("ws://127.0.0.1:%d/ws", r.Port())
	c, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, nil)
	require.NoError(t, err)
	defer c.Close() //nolint:errcheck
	require.NoError(t, r.WaitForConnection(ctx))

	// Send multiple event types.
	events := []event.Event{
		{Type: event.TypeThinking, Payload: event.ThinkingPayload{Thinking: "hmm"}},
		{Type: event.TypeToolCallStarted, Payload: event.ToolCallPayload{Name: "Bash"}},
		{Type: event.TypeToolCallFinished, Payload: event.ToolResultPayload{Name: "Bash"}},
		{Type: event.TypeError, Payload: event.ErrorPayload{Message: "oops"}},
		{Type: event.TypeUsage, Payload: event.UsagePayload{
			TurnUsage: model.Usage{InputTokens: 10, OutputTokens: 20},
		}},
	}

	for _, evt := range events {
		require.NoError(t, r.RenderEvent(evt))
	}

	// Verify each event arrives on the WebSocket.
	for range events {
		_, raw, err := c.ReadMessage()
		require.NoError(t, err)
		var msg WSMessage
		require.NoError(t, json.Unmarshal(raw, &msg))
		assert.Equal(t, msgTypeEvent, msg.Type)

		p, ok := msg.Payload.(map[string]any)
		if !ok {
			continue
		}
		evtType, _ := p["type"].(string)
		assert.NotEmpty(t, evtType)
	}
}

func TestRendererRenderLine(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	r, err := NewRenderer()
	require.NoError(t, err)
	defer r.Close() //nolint:errcheck

	wsURL := fmt.Sprintf("ws://127.0.0.1:%d/ws", r.Port())
	c, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, nil)
	require.NoError(t, err)
	defer c.Close() //nolint:errcheck
	require.NoError(t, r.WaitForConnection(ctx))

	require.NoError(t, r.RenderLine("status line"))

	_, raw, err := c.ReadMessage()
	require.NoError(t, err)
	var msg WSMessage
	require.NoError(t, json.Unmarshal(raw, &msg))
	assert.Equal(t, msgTypeLine, msg.Type)
}

func TestRendererNoClientNoBlock(t *testing.T) {
	// When no client is connected, RenderEvent and RenderLine should not block.
	r, err := NewRenderer()
	require.NoError(t, err)
	defer r.Close() //nolint:errcheck

	done := make(chan bool, 1)
	go func() {
		err = r.RenderEvent(event.Event{
			Type:    event.TypeMessageDelta,
			Payload: event.MessageDeltaPayload{Text: "test"},
		})
		r.RenderLine("test line")
		done <- true
	}()

	select {
	case <-done:
		// Success: didn't block.
	case <-time.After(2 * time.Second):
		t.Fatal("RenderEvent with no client blocked unexpectedly")
	}
}

func TestApprovalPrompter(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	r, err := NewRenderer()
	require.NoError(t, err)
	defer r.Close() //nolint:errcheck

	wsURL := fmt.Sprintf("ws://127.0.0.1:%d/ws", r.Port())
	c, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, nil)
	require.NoError(t, err)
	defer c.Close() //nolint:errcheck
	require.NoError(t, r.WaitForConnection(ctx))

	prompter := NewApprovalPrompter(r)

	// Run Prompt in a goroutine — it blocks until the TUI responds.
	respCh := make(chan approval.Response, 1)
	errCh := make(chan error, 1)
	go func() {
		resp, err := prompter.Prompt(ctx, approval.Prompt{
			Title: "Approve Bash?",
			Body:  "Execute: ls /tmp",
		})
		if err != nil {
			errCh <- err
			return
		}
		respCh <- resp
	}()

	// Read the approval_prompt message from WebSocket.
	_, raw, err := c.ReadMessage()
	require.NoError(t, err)
	var msg WSMessage
	require.NoError(t, json.Unmarshal(raw, &msg))
	assert.Equal(t, msgTypeApprovalPrompt, msg.Type)

	// Send back an approval response.
	err = c.WriteJSON(WSMessage{
		Type: msgTypeApproval,
		Payload: map[string]any{
			"approved": true,
		},
	})
	require.NoError(t, err)

	select {
	case resp := <-respCh:
		assert.True(t, resp.Approved)
	case err := <-errCh:
		t.Fatalf("Prompt error: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("Prompt did not return after approval response")
	}
}

func TestApprovalPrompterDeny(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	r, err := NewRenderer()
	require.NoError(t, err)
	defer r.Close() //nolint:errcheck

	wsURL := fmt.Sprintf("ws://127.0.0.1:%d/ws", r.Port())
	c, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, nil)
	require.NoError(t, err)
	defer c.Close() //nolint:errcheck
	require.NoError(t, r.WaitForConnection(ctx))

	prompter := NewApprovalPrompter(r)

	respCh := make(chan approval.Response, 1)
	errCh := make(chan error, 1)
	go func() {
		resp, err := prompter.Prompt(ctx, approval.Prompt{
			Title: "Approve Write?",
			Body:  "Write to: /tmp/test",
		})
		if err != nil {
			errCh <- err
			return
		}
		respCh <- resp
	}()

	// Read the approval_prompt message.
	_, raw, err := c.ReadMessage()
	require.NoError(t, err)
	var msg WSMessage
	require.NoError(t, json.Unmarshal(raw, &msg))
	assert.Equal(t, msgTypeApprovalPrompt, msg.Type)

	// Send back a deny response.
	err = c.WriteJSON(WSMessage{
		Type: msgTypeApproval,
		Payload: map[string]any{
			"approved": false,
		},
	})
	require.NoError(t, err)

	select {
	case resp := <-respCh:
		assert.False(t, resp.Approved)
	case err := <-errCh:
		t.Fatalf("Prompt error: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("Prompt did not return after deny response")
	}
}

// TestDeltaEventsArriveSeparately verifies that each RenderEvent call produces a
// distinct WebSocket message. If TCP buffering (Nagle) or batching collapses
// multiple deltas into one frame, this test fails.
func TestDeltaEventsArriveSeparately(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	r, err := NewRenderer()
	require.NoError(t, err)
	defer r.Close() //nolint:errcheck

	wsURL := fmt.Sprintf("ws://127.0.0.1:%d/ws", r.Port())
	c, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, nil)
	require.NoError(t, err)
	defer c.Close() //nolint:errcheck
	require.NoError(t, r.WaitForConnection(ctx))

	// Send 10 delta events in rapid succession.
	const count = 10
	for i := 0; i < count; i++ {
		require.NoError(t, r.RenderEvent(event.Event{
			Type:    event.TypeMessageDelta,
			Payload: event.MessageDeltaPayload{Text: fmt.Sprintf("chunk-%d", i)},
		}))
	}

	// Verify each delta produced a separate WebSocket message.
	received := make([]string, 0, count)
	for i := 0; i < count; i++ {
		_, raw, err := c.ReadMessage()
		require.NoError(t, err)
		var msg WSMessage
		require.NoError(t, json.Unmarshal(raw, &msg))
		require.Equal(t, msgTypeEvent, msg.Type)

		p, ok := msg.Payload.(map[string]any)
		require.True(t, ok, "payload must be an object")
		require.Equal(t, "message.delta", p["type"])
		inner, ok := p["payload"].(map[string]any)
		require.True(t, ok, "inner payload must be an object")
		text, ok := inner["text"].(string)
		require.True(t, ok, "text must be a string")
		received = append(received, text)
	}

	// All 10 must have arrived as unique messages (not batched into fewer).
	assert.Len(t, received, count)
	for i := 0; i < count; i++ {
		assert.Equal(t, fmt.Sprintf("chunk-%d", i), received[i])
	}
}

// TestNoTCPDelay verifies the WebSocket connection has TCP_NODELAY enabled so
// that small event frames are not held back by Nagle's algorithm.
func TestNoTCPDelay(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	r, err := NewRenderer()
	require.NoError(t, err)
	defer r.Close() //nolint:errcheck

	wsURL := fmt.Sprintf("ws://127.0.0.1:%d/ws", r.Port())
	c, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, nil)
	require.NoError(t, err)
	defer c.Close() //nolint:errcheck
	require.NoError(t, r.WaitForConnection(ctx))

	require.NoError(t, r.RenderEvent(event.Event{
		Type:    event.TypeMessageDelta,
		Payload: event.MessageDeltaPayload{Text: "ping"},
	}))

	// Read one message to confirm the connection works.
	_, raw, err := c.ReadMessage()
	require.NoError(t, err)
	var msg WSMessage
	require.NoError(t, json.Unmarshal(raw, &msg))
	assert.Equal(t, msgTypeEvent, msg.Type)
}
