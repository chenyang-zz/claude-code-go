package client

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

type bufferWriteCloser struct {
	bytes.Buffer
}

func (b *bufferWriteCloser) Close() error { return nil }

// mockStdioServer prints a JSON-RPC response for each line read from stdin.
func TestStdioClientTransportSendAndReceive(t *testing.T) {
	// Use a simple echo script that replies with the request ID.
	transport, err := NewStdioClientTransport("sh", []string{"-c", `
		while IFS= read -r line; do
			id=$(echo "$line" | sed 's/.*"id":"\([^"]*\)".*/\1/')
			echo '{"jsonrpc":"2.0","id":"'"$id"'","result":{"ok":true}}'
		done
	`}, nil)
	if err != nil {
		t.Fatalf("create transport: %v", err)
	}
	defer transport.Close()

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      "42",
		Method:  "test",
		Params:  json.RawMessage(`{}`),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := transport.Send(ctx, req)
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if resp.ID != "42" {
		t.Fatalf("response id = %q, want 42", resp.ID)
	}
}

func TestStdioClientTransportClosed(t *testing.T) {
	transport, err := NewStdioClientTransport("sh", []string{"-c", "exit 0"}, nil)
	if err != nil {
		t.Fatalf("create transport: %v", err)
	}
	transport.Close()

	req := JSONRPCRequest{JSONRPC: "2.0", ID: "1", Method: "test"}
	ctx := context.Background()
	_, err = transport.Send(ctx, req)
	if err == nil {
		t.Fatal("expected error after close")
	}
}

func TestStdioClientTransportTimeout(t *testing.T) {
	// Server that never replies.
	transport, err := NewStdioClientTransport("sh", []string{"-c", "cat >/dev/null"}, nil)
	if err != nil {
		t.Fatalf("create transport: %v", err)
	}
	defer transport.Close()

	req := JSONRPCRequest{JSONRPC: "2.0", ID: "99", Method: "hang"}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err = transport.Send(ctx, req)
	if err != context.DeadlineExceeded {
		t.Fatalf("err = %v, want deadline exceeded", err)
	}
}

func TestStdioClientTransportNextID(t *testing.T) {
	transport := &StdioClientTransport{}
	id1 := transport.NextID()
	id2 := transport.NextID()
	if id1 == "" || id2 == "" {
		t.Fatal("NextID returned empty")
	}
	if id1 == id2 {
		t.Fatalf("ids should differ: %q vs %q", id1, id2)
	}
}

func TestStdioClientTransportHandlesIncomingRequest(t *testing.T) {
	transport := &StdioClientTransport{
		stdin:                &bufferWriteCloser{},
		pending:              make(map[RequestID]chan JSONRPCResponse),
		notificationHandlers: make(map[string]NotificationHandler),
		requestHandlers:      make(map[string]RequestHandler),
	}

	done := make(chan struct{}, 1)
	transport.SetRequestHandler(ElicitRequestMethod, func(req JSONRPCRequest) (any, error) {
		done <- struct{}{}
		return ElicitResult{Action: "cancel"}, nil
	})

	transport.handleRequest(JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      "elic-1",
		Method:  ElicitRequestMethod,
		Params:  json.RawMessage(`{"mode":"form","message":"Need input"}`),
	})

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for request handler")
	}

	deadline := time.Now().Add(time.Second)
	for {
		written := transport.stdin.(*bufferWriteCloser).String()
		if strings.Contains(written, `"id":"elic-1"`) && strings.Contains(written, `"action":"cancel"`) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("written response = %q, want request id and cancel action", written)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
