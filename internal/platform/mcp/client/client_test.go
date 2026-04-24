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
	responses map[RequestID]JSONRPCResponse
}

func (m *mockTransport) Send(ctx context.Context, req JSONRPCRequest) (*JSONRPCResponse, error) {
	resp, ok := m.responses[req.ID]
	if !ok {
		return nil, nil
	}
	return &resp, nil
}

func (m *mockTransport) Close() error { return nil }

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

func (m *blockingMockTransport) Close() error { return nil }
