package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestNewHTTPClientTransportRoundTrip verifies that the minimal HTTP transport can
// complete the MCP handshake and snapshot requests.
func TestNewHTTPClientTransportRoundTrip(t *testing.T) {
	t.Parallel()

	server := newHTTPTestServer(t)
	defer server.Close()

	transport, err := NewHTTPClientTransport(context.Background(), server.URL, map[string]string{"X-Test": "ok"})
	if err != nil {
		t.Fatalf("NewHTTPClientTransport() error = %v", err)
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
	if result.ServerInfo.Name != "http-test" {
		t.Fatalf("serverInfo.name = %q, want http-test", result.ServerInfo.Name)
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

// TestNewHTTPClientTransportRejectsInvalidScheme verifies non-HTTP endpoints fail fast.
func TestNewHTTPClientTransportRejectsInvalidScheme(t *testing.T) {
	t.Parallel()

	if _, err := NewHTTPClientTransport(context.Background(), "ws://example.com/mcp", nil); err == nil {
		t.Fatal("NewHTTPClientTransport() error = nil, want invalid scheme error")
	}
}

// newHTTPTestServer returns a tiny JSON-RPC server used by transport round-trip tests.
func newHTTPTestServer(t *testing.T) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if got := r.Header.Get("X-Test"); got != "ok" {
			t.Fatalf("X-Test header = %q, want ok", got)
		}
		if got := r.Header.Get("Accept"); got != "application/json, text/event-stream" {
			t.Fatalf("Accept header = %q, want streamable HTTP accept", got)
		}

		var req JSONRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}

		resp := JSONRPCResponse{JSONRPC: "2.0", ID: req.ID}
		switch req.Method {
		case "initialize":
			resp.Result = json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{"tools":{"listChanged":false},"resources":{"listChanged":false},"prompts":{"listChanged":false}},"serverInfo":{"name":"http-test","version":"1.0"}}`)
		case "tools/list":
			resp.Result = json.RawMessage(`{"tools":[{"name":"tool_one","description":"one"}]}`)
		case "resources/list":
			resp.Result = json.RawMessage(`{"resources":[{"uri":"file:///tmp/a","name":"config"}]}`)
		case "prompts/list":
			resp.Result = json.RawMessage(`{"prompts":[{"name":"summarize","description":"Summarize"}]}`)
		default:
			resp.Error = &JSONRPCError{
				Code:    -32601,
				Message: fmt.Sprintf("unknown method %q", req.Method),
			}
		}

		payload, err := json.Marshal(resp)
		if err != nil {
			t.Fatalf("marshal response: %v", err)
		}
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write(payload)
	}))
}
