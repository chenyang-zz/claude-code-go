package client

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

// mockTransport implements Transport for testing.
type mockTransport struct {
	responses            map[RequestID]JSONRPCResponse
	notificationHandlers map[string]NotificationHandler
}

func (m *mockTransport) Send(ctx context.Context, req JSONRPCRequest) (*JSONRPCResponse, error) {
	resp, ok := m.responses[req.ID]
	if !ok {
		return nil, nil
	}
	return &resp, nil
}

func (m *mockTransport) Close() error { return nil }

func (m *mockTransport) SetNotificationHandler(method string, handler NotificationHandler) {
	if m.notificationHandlers == nil {
		m.notificationHandlers = make(map[string]NotificationHandler)
	}
	if handler == nil {
		delete(m.notificationHandlers, method)
		return
	}
	m.notificationHandlers[method] = handler
}

func (m *mockTransport) emitNotification(method string, params json.RawMessage) {
	if handler := m.notificationHandlers[method]; handler != nil {
		handler(JSONRPCNotification{
			JSONRPC: "2.0",
			Method:  method,
			Params:  params,
		})
	}
}

func TestClientInitialize(t *testing.T) {
	mt := &mockTransport{
		responses: map[RequestID]JSONRPCResponse{
			"1": {
				JSONRPC: "2.0",
				ID:      "1",
				Result:  json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"serverInfo":{"name":"test","version":"1.0"}}`),
			},
		},
	}
	c := NewClient(mt)
	result, err := c.Initialize(context.Background(), InitializeRequest{
		ProtocolVersion: "2024-11-05",
		ClientInfo:      Implementation{Name: "client", Version: "0.1"},
	})
	if err != nil {
		t.Fatalf("initialize: %v", err)
	}
	if result.ServerInfo.Name != "test" {
		t.Fatalf("serverInfo.name = %q", result.ServerInfo.Name)
	}
}

func TestClientInitializeError(t *testing.T) {
	mt := &mockTransport{
		responses: map[RequestID]JSONRPCResponse{
			"1": {
				JSONRPC: "2.0",
				ID:      "1",
				Error:   &JSONRPCError{Code: -32600, Message: "Invalid Request"},
			},
		},
	}
	c := NewClient(mt)
	_, err := c.Initialize(context.Background(), InitializeRequest{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestClientListTools(t *testing.T) {
	mt := &mockTransport{
		responses: map[RequestID]JSONRPCResponse{
			"1": {
				JSONRPC: "2.0",
				ID:      "1",
				Result:  json.RawMessage(`{"tools":[{"name":"read_file","description":"Read a file"}]}`),
			},
		},
	}
	c := NewClient(mt)
	result, err := c.ListTools(context.Background())
	if err != nil {
		t.Fatalf("listTools: %v", err)
	}
	if len(result.Tools) != 1 || result.Tools[0].Name != "read_file" {
		t.Fatalf("tools = %+v", result.Tools)
	}
}

func TestClientListResources(t *testing.T) {
	mt := &mockTransport{
		responses: map[RequestID]JSONRPCResponse{
			"1": {
				JSONRPC: "2.0",
				ID:      "1",
				Result:  json.RawMessage(`{"resources":[{"uri":"file:///tmp/a","name":"config","description":"Config file"}]}`),
			},
		},
	}
	c := NewClient(mt)
	result, err := c.ListResources(context.Background())
	if err != nil {
		t.Fatalf("listResources: %v", err)
	}
	if len(result.Resources) != 1 || result.Resources[0].URI != "file:///tmp/a" {
		t.Fatalf("resources = %+v", result.Resources)
	}
}

func TestClientReadResource(t *testing.T) {
	mt := &mockTransport{
		responses: map[RequestID]JSONRPCResponse{
			"1": {
				JSONRPC: "2.0",
				ID:      "1",
				Result:  json.RawMessage(`{"contents":[{"uri":"file:///tmp/a","mimeType":"text/plain","text":"hello"}]}`),
			},
		},
	}
	c := NewClient(mt)
	result, err := c.ReadResource(context.Background(), ReadResourceRequest{URI: "file:///tmp/a"})
	if err != nil {
		t.Fatalf("readResource: %v", err)
	}
	if len(result.Contents) != 1 {
		t.Fatalf("contents = %+v", result.Contents)
	}
}

func TestClientListPrompts(t *testing.T) {
	mt := &mockTransport{
		responses: map[RequestID]JSONRPCResponse{
			"1": {
				JSONRPC: "2.0",
				ID:      "1",
				Result:  json.RawMessage(`{"prompts":[{"name":"summarize","description":"Summarize","arguments":{"path":{"name":"path","description":"Target file","required":true}}}]}`),
			},
		},
	}
	c := NewClient(mt)
	result, err := c.ListPrompts(context.Background())
	if err != nil {
		t.Fatalf("listPrompts: %v", err)
	}
	if len(result.Prompts) != 1 || result.Prompts[0].Name != "summarize" {
		t.Fatalf("prompts = %+v", result.Prompts)
	}
}

func TestClientGetPrompt(t *testing.T) {
	mt := &mockTransport{
		responses: map[RequestID]JSONRPCResponse{
			"1": {
				JSONRPC: "2.0",
				ID:      "1",
				Result:  json.RawMessage(`{"description":"Summarize","messages":[{"role":"user","content":[{"type":"text","text":"Hello"}]}]}`),
			},
		},
	}
	c := NewClient(mt)
	result, err := c.GetPrompt(context.Background(), GetPromptRequest{Name: "summarize"})
	if err != nil {
		t.Fatalf("getPrompt: %v", err)
	}
	if len(result.Messages) != 1 {
		t.Fatalf("messages = %+v", result.Messages)
	}
}

func TestClientCallTool(t *testing.T) {
	mt := &mockTransport{
		responses: map[RequestID]JSONRPCResponse{
			"1": {
				JSONRPC: "2.0",
				ID:      "1",
				Result:  json.RawMessage(`{"content":[{"type":"text","text":"done"}],"isError":false}`),
			},
		},
	}
	c := NewClient(mt)
	result, err := c.CallTool(context.Background(), CallToolRequest{Name: "test", Arguments: map[string]any{}})
	if err != nil {
		t.Fatalf("callTool: %v", err)
	}
	if len(result.Content) != 1 || result.Content[0].Text != "done" {
		t.Fatalf("content = %+v", result.Content)
	}
}

func TestClientClose(t *testing.T) {
	mt := &mockTransport{}
	c := NewClient(mt)
	if err := c.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	// Subsequent calls should fail.
	_, err := c.ListTools(context.Background())
	if err == nil {
		t.Fatal("expected error after close")
	}
}

func TestClientNotificationHandler(t *testing.T) {
	mt := &mockTransport{}
	c := NewClient(mt)

	received := make(chan JSONRPCNotification, 1)
	if err := c.SetNotificationHandler("tools/list_changed", func(n JSONRPCNotification) {
		received <- n
	}); err != nil {
		t.Fatalf("SetNotificationHandler: %v", err)
	}

	mt.emitNotification("tools/list_changed", json.RawMessage(`{"foo":"bar"}`))

	select {
	case n := <-received:
		if n.Method != "tools/list_changed" {
			t.Fatalf("notification method = %q", n.Method)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for notification")
	}
}

func TestStdioClientTransportDispatchesNotification(t *testing.T) {
	transport, err := NewStdioClientTransport("sh", []string{"-c", `
		while IFS= read -r line; do
			id=$(printf '%s' "$line" | sed -n 's/.*"id":"\([^"]*\)".*/\1/p')
			method=$(printf '%s' "$line" | sed -n 's/.*"method":"\([^"]*\)".*/\1/p')
			case "$method" in
				initialize)
					printf '%s\n' '{"jsonrpc":"2.0","method":"tools/list_changed"}'
					printf '%s\n' '{"jsonrpc":"2.0","id":"'"$id"'","result":{"protocolVersion":"2024-11-05","capabilities":{"tools":{"listChanged":true}},"serverInfo":{"name":"test","version":"1.0"}}}'
					;;
			esac
		done
	`}, nil)
	if err != nil {
		t.Fatalf("NewStdioClientTransport: %v", err)
	}
	defer transport.Close()

	c := NewClient(transport)
	received := make(chan struct{}, 1)
	if err := c.SetNotificationHandler("tools/list_changed", func(JSONRPCNotification) {
		select {
		case received <- struct{}{}:
		default:
		}
	}); err != nil {
		t.Fatalf("SetNotificationHandler: %v", err)
	}

	if _, err := c.Initialize(context.Background(), InitializeRequest{
		ProtocolVersion: "2024-11-05",
		ClientInfo:      Implementation{Name: "client", Version: "0.1"},
	}); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	select {
	case <-received:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for notification dispatch")
	}
}

func TestClientRequestTimeout(t *testing.T) {
	// mockTransport that blocks until context cancellation.
	mt := &blockingMockTransport{}
	c := NewClient(mt)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := c.ListTools(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want deadline exceeded", err)
	}
}

// blockingMockTransport waits for context cancellation.
type blockingMockTransport struct{}

func (m *blockingMockTransport) Send(ctx context.Context, req JSONRPCRequest) (*JSONRPCResponse, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func (m *blockingMockTransport) SetNotificationHandler(method string, handler NotificationHandler) {}

func (m *blockingMockTransport) Close() error { return nil }
