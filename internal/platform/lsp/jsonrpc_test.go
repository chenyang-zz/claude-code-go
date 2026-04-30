package lsp

import (
	"bufio"
	"bytes"
	"strings"
	"testing"
)

func TestWriteAndReadMessage(t *testing.T) {
	var buf bytes.Buffer

	req := Request{
		JSONRPC: jsonrpcVersion,
		ID:      1,
		Method:  "test/method",
		Params:  map[string]any{"key": "value"},
	}

	if err := WriteMessage(&buf, req); err != nil {
		t.Fatalf("WriteMessage error: %v", err)
	}

	encoded := buf.String()
	if !strings.HasPrefix(encoded, "Content-Length: ") {
		t.Error("message should start with Content-Length header")
	}

	var decoded Request
	if err := ReadMessage(bufio.NewReader(&buf), &decoded); err != nil {
		t.Fatalf("ReadMessage error: %v", err)
	}

	if decoded.ID != req.ID {
		t.Errorf("expected id %d, got %d", req.ID, decoded.ID)
	}
	if decoded.Method != req.Method {
		t.Errorf("expected method %q, got %q", req.Method, decoded.Method)
	}
	if decoded.Params["key"] != "value" {
		t.Errorf("expected params.key='value', got %v", decoded.Params["key"])
	}
}

func TestWriteNotification(t *testing.T) {
	var buf bytes.Buffer

	notif := NewNotification("textDocument/didOpen", map[string]any{
		"textDocument": map[string]any{"uri": "file:///test.go"},
	})

	if err := WriteMessage(&buf, notif); err != nil {
		t.Fatalf("WriteMessage error: %v", err)
	}

	var decoded Notification
	if err := ReadMessage(bufio.NewReader(&buf), &decoded); err != nil {
		t.Fatalf("ReadMessage error: %v", err)
	}

	if decoded.Method != "textDocument/didOpen" {
		t.Errorf("expected method %q, got %q", "textDocument/didOpen", decoded.Method)
	}
}

func TestNewRequestHasUniqueIDs(t *testing.T) {
	r1 := NewRequest("method1", nil)
	r2 := NewRequest("method2", nil)

	if r1.ID == r2.ID {
		t.Error("request IDs should be unique")
	}
	if r1.JSONRPC != jsonrpcVersion {
		t.Errorf("expected jsonrpc %q, got %q", jsonrpcVersion, r1.JSONRPC)
	}
}

func TestWriteMessage_LargePayload(t *testing.T) {
	var buf bytes.Buffer

	largeParams := map[string]any{
		"data": strings.Repeat("x", 5000),
	}
	req := NewRequest("large/method", largeParams)

	if err := WriteMessage(&buf, req); err != nil {
		t.Fatalf("WriteMessage error: %v", err)
	}

	var decoded Request
	if err := ReadMessage(bufio.NewReader(&buf), &decoded); err != nil {
		t.Fatalf("ReadMessage error: %v", err)
	}
}

func TestReadMessageEmpty(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader(""))
	var resp Response
	if err := ReadMessage(reader, &resp); err == nil {
		t.Error("expected error for empty input")
	}
}
