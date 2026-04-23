package remote

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// TestDialWebSocketRecv verifies ws connections emit normalized text events.
func TestDialWebSocketRecv(t *testing.T) {
	t.Parallel()

	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("Upgrade() error = %v", err)
		}
		defer conn.Close()

		if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"message"}`)); err != nil {
			t.Fatalf("WriteMessage() error = %v", err)
		}
	}))
	defer server.Close()

	wsURL := strings.Replace(server.URL, "http://", "ws://", 1)
	client, err := DialWebSocket(context.Background(), wsURL, nil, nil)
	if err != nil {
		t.Fatalf("DialWebSocket() error = %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	event, err := client.Recv(ctx)
	if err != nil {
		t.Fatalf("Recv() error = %v", err)
	}
	if event.Transport != TransportWebSocket {
		t.Fatalf("event.Transport = %q, want %q", event.Transport, TransportWebSocket)
	}
	if event.Type != "text" {
		t.Fatalf("event.Type = %q, want %q", event.Type, "text")
	}
	if got := string(event.Data); got != `{"type":"message"}` {
		t.Fatalf("event.Data = %q, want json payload", got)
	}
}

// TestDialWebSocketRejectsInvalidScheme verifies non-ws endpoints fail fast.
func TestDialWebSocketRejectsInvalidScheme(t *testing.T) {
	t.Parallel()

	_, err := DialWebSocket(context.Background(), "https://example.com/ws", nil, nil)
	if err == nil {
		t.Fatalf("DialWebSocket() error = nil, want invalid scheme error")
	}
}

// TestWebSocketSend verifies Send delivers text messages to the server.
func TestWebSocketSend(t *testing.T) {
	t.Parallel()

	upgrader := websocket.Upgrader{}
	received := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("Upgrade() error = %v", err)
		}
		defer conn.Close()

		_, msg, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("ReadMessage() error = %v", err)
		}
		received <- string(msg)
	}))
	defer server.Close()

	wsURL := strings.Replace(server.URL, "http://", "ws://", 1)
	client, err := DialWebSocket(context.Background(), wsURL, nil, nil)
	if err != nil {
		t.Fatalf("DialWebSocket() error = %v", err)
	}
	defer client.Close()

	payload := `{"type":"control_response","response":{"subtype":"success","request_id":"r1"}}`
	if err := client.Send([]byte(payload)); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	select {
	case got := <-received:
		if got != payload {
			t.Fatalf("server received = %q, want %q", got, payload)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for server to receive message")
	}
}

// TestWebSocketSendAfterClose verifies Send returns ErrStreamClosed after Close.
func TestWebSocketSendAfterClose(t *testing.T) {
	t.Parallel()

	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("Upgrade() error = %v", err)
		}
		defer conn.Close()
	}))
	defer server.Close()

	wsURL := strings.Replace(server.URL, "http://", "ws://", 1)
	client, err := DialWebSocket(context.Background(), wsURL, nil, nil)
	if err != nil {
		t.Fatalf("DialWebSocket() error = %v", err)
	}
	_ = client.Close()

	if err := client.Send([]byte(`{"type":"test"}`)); !errors.Is(err, ErrStreamClosed) {
		t.Fatalf("Send() after Close error = %v, want ErrStreamClosed", err)
	}
}
