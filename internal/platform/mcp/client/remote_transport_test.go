package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gorilla/websocket"
)

// TestNewSSEClientTransportRoundTrip verifies that the remote SSE transport can
// complete the MCP handshake and snapshot requests.
func TestNewSSEClientTransportRoundTrip(t *testing.T) {
	t.Parallel()

	server := newRemoteSSETestServer(t)
	defer server.Close()

	transport, err := NewSSEClientTransport(context.Background(), server.URL, map[string]string{"X-Test": "ok"})
	if err != nil {
		t.Fatalf("NewSSEClientTransport() error = %v", err)
	}
	defer transport.Close()

	c := NewClient(transport)
	result, err := c.Initialize(context.Background(), InitializeRequest{
		ProtocolVersion: "2024-11-05",
		Capabilities: ClientCapabilities{
			Roots: map[string]any{},
		},
		ClientInfo: Implementation{
			Name:    "claude-code-go",
			Version: "0.1.0",
		},
	})
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	if result.ServerInfo.Name != "sse-test" {
		t.Fatalf("serverInfo.name = %q, want sse-test", result.ServerInfo.Name)
	}

	if _, err := c.ListTools(context.Background()); err != nil {
		t.Fatalf("ListTools() error = %v", err)
	}
	if _, err := c.ListResources(context.Background()); err != nil {
		t.Fatalf("ListResources() error = %v", err)
	}
	if _, err := c.ListPrompts(context.Background()); err != nil {
		t.Fatalf("ListPrompts() error = %v", err)
	}
}

// TestNewWebSocketClientTransportRoundTrip verifies that the remote WebSocket
// transport can complete the MCP handshake and snapshot requests.
func TestNewWebSocketClientTransportRoundTrip(t *testing.T) {
	t.Parallel()

	server := newRemoteWebSocketTestServer(t)
	defer server.Close()

	wsURL := strings.Replace(server.URL, "http://", "ws://", 1)
	transport, err := NewWebSocketClientTransport(context.Background(), wsURL, map[string]string{"X-Test": "ok"})
	if err != nil {
		t.Fatalf("NewWebSocketClientTransport() error = %v", err)
	}
	defer transport.Close()

	c := NewClient(transport)
	result, err := c.Initialize(context.Background(), InitializeRequest{
		ProtocolVersion: "2024-11-05",
		Capabilities: ClientCapabilities{
			Roots: map[string]any{},
		},
		ClientInfo: Implementation{
			Name:    "claude-code-go",
			Version: "0.1.0",
		},
	})
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	if result.ServerInfo.Name != "ws-test" {
		t.Fatalf("serverInfo.name = %q, want ws-test", result.ServerInfo.Name)
	}

	if _, err := c.ListTools(context.Background()); err != nil {
		t.Fatalf("ListTools() error = %v", err)
	}
	if _, err := c.ListResources(context.Background()); err != nil {
		t.Fatalf("ListResources() error = %v", err)
	}
	if _, err := c.ListPrompts(context.Background()); err != nil {
		t.Fatalf("ListPrompts() error = %v", err)
	}
}

func newRemoteSSETestServer(t *testing.T) *httptest.Server {
	t.Helper()

	var once sync.Once
	events := make(chan []byte, 16)

	send := func(resp JSONRPCResponse) {
		payload, err := json.Marshal(resp)
		if err != nil {
			t.Fatalf("marshal response: %v", err)
		}
		events <- payload
	}

	serveResponse := func(req JSONRPCRequest) JSONRPCResponse {
		switch req.Method {
		case "initialize":
			return JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{"tools":{"listChanged":false},"resources":{"listChanged":false},"prompts":{"listChanged":false}},"serverInfo":{"name":"sse-test","version":"1.0"}}`),
			}
		case "tools/list":
			return JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  json.RawMessage(`{"tools":[{"name":"tool_one","description":"one"}]}`),
			}
		case "resources/list":
			return JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  json.RawMessage(`{"resources":[{"uri":"file:///tmp/a","name":"config"}]}`),
			}
		case "prompts/list":
			return JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  json.RawMessage(`{"prompts":[{"name":"summarize","description":"Summarize"}]}`),
			}
		default:
			return JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error: &JSONRPCError{
					Code:    -32601,
					Message: fmt.Sprintf("unknown method %q", req.Method),
				},
			}
		}
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("content-type", "text/event-stream")
			flusher, ok := w.(http.Flusher)
			if !ok {
				t.Fatalf("response writer does not support flushing")
			}
			_, _ = fmt.Fprintln(w, ": ok")
			flusher.Flush()
			for {
				select {
				case payload := <-events:
					_, _ = fmt.Fprintf(w, "data: %s\n\n", payload)
					flusher.Flush()
				case <-r.Context().Done():
					return
				}
			}
		case http.MethodPost:
			var req JSONRPCRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			once.Do(func() {})
			send(serveResponse(req))
			w.WriteHeader(http.StatusAccepted)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))

	return server
}

func newRemoteWebSocketTestServer(t *testing.T) *httptest.Server {
	t.Helper()

	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("upgrade: %v", err)
		}
		defer conn.Close()

		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var req JSONRPCRequest
			if err := json.Unmarshal(msg, &req); err != nil {
				t.Fatalf("unmarshal request: %v", err)
			}
			resp := JSONRPCResponse{JSONRPC: "2.0", ID: req.ID}
			switch req.Method {
			case "initialize":
				resp.Result = json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{"tools":{"listChanged":false},"resources":{"listChanged":false},"prompts":{"listChanged":false}},"serverInfo":{"name":"ws-test","version":"1.0"}}`)
			case "tools/list":
				resp.Result = json.RawMessage(`{"tools":[{"name":"tool_one","description":"one"}]}`)
			case "resources/list":
				resp.Result = json.RawMessage(`{"resources":[{"uri":"file:///tmp/a","name":"config"}]}`)
			case "prompts/list":
				resp.Result = json.RawMessage(`{"prompts":[{"name":"summarize","description":"Summarize"}]}`)
			default:
				resp.Error = &JSONRPCError{Code: -32601, Message: fmt.Sprintf("unknown method %q", req.Method)}
			}
			payload, err := json.Marshal(resp)
			if err != nil {
				t.Fatalf("marshal response: %v", err)
			}
			if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
				t.Fatalf("write response: %v", err)
			}
		}
	}))

	return server
}
