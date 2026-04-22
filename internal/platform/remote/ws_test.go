package remote

import (
	"context"
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
