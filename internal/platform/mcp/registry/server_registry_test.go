package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sheepzhao/claude-code-go/internal/platform/mcp/client"
)

func TestNewServerRegistry(t *testing.T) {
	r := NewServerRegistry()
	if r == nil {
		t.Fatal("NewServerRegistry returned nil")
	}
	if len(r.List()) != 0 {
		t.Fatal("new registry should be empty")
	}
}

func TestServerRegistryLoadConfigs(t *testing.T) {
	r := NewServerRegistry()
	r.LoadConfigs(map[string]client.ServerConfig{
		"fs": {Command: "npx", Args: []string{"-y", "@modelcontextprotocol/server-filesystem"}},
	})
	entries := r.List()
	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}
	if entries[0].Name != "fs" {
		t.Fatalf("name = %q", entries[0].Name)
	}
	if entries[0].Status != StatusDisabled {
		t.Fatalf("status = %q, want disabled", entries[0].Status)
	}
}

func TestServerRegistryConnectAllUnsupportedType(t *testing.T) {
	r := NewServerRegistry()
	r.LoadConfigs(map[string]client.ServerConfig{
		"proxy": {Type: "sdk", URL: "https://example.com/mcp"},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	r.ConnectAll(ctx)

	entries := r.List()
	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}
	if entries[0].Status != StatusFailed {
		t.Fatalf("status = %q, want failed", entries[0].Status)
	}
}

func TestServerRegistryConnectAllClaudeAiProxyNeedsAuth(t *testing.T) {
	t.Parallel()

	r := NewServerRegistry()
	r.LoadConfigs(map[string]client.ServerConfig{
		"proxy": {Type: "claudeai-proxy", URL: "https://example.com/mcp"},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	r.ConnectAll(ctx)

	entries := r.List()
	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}
	if entries[0].Status != StatusNeedsAuth {
		t.Fatalf("status = %q, want needs-auth", entries[0].Status)
	}
	if !strings.Contains(entries[0].Error, "authentication required") {
		t.Fatalf("error = %q, want authentication hint", entries[0].Error)
	}
}

func TestServerRegistryConnectAllClaudeAiProxyWithAuthToken(t *testing.T) {
	t.Parallel()

	server := newRegistryClaudeAiProxyServer(t, "token-123")
	defer server.Close()

	r := NewServerRegistry()
	r.SetAuthToken("token-123")
	r.LoadConfigs(map[string]client.ServerConfig{
		"proxy": {Type: "claudeai-proxy", URL: server.URL, Headers: map[string]string{"X-Test": "ok"}},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	r.ConnectAll(ctx)

	entries := r.List()
	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}
	if entries[0].Status != StatusConnected {
		t.Fatalf("status = %q, want connected", entries[0].Status)
	}
	if len(entries[0].Tools) != 1 || entries[0].Tools[0].Name != "tool_one" {
		t.Fatalf("tools = %#v", entries[0].Tools)
	}
}

func TestServerRegistryReconnectServerClaudeAiProxyAfterAuthToken(t *testing.T) {
	t.Parallel()

	server := newRegistryClaudeAiProxyServer(t, "token-123")
	defer server.Close()

	r := NewServerRegistry()
	r.LoadConfigs(map[string]client.ServerConfig{
		"proxy": {Type: "claudeai-proxy", URL: server.URL, Headers: map[string]string{"X-Test": "ok"}},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	r.ConnectAll(ctx)

	before, ok := r.GetEntry("proxy")
	if !ok || before.Status != StatusNeedsAuth {
		t.Fatalf("before status = %#v, want needs-auth", before.Status)
	}

	r.SetAuthToken("token-123")
	if err := r.ReconnectServer(ctx, "proxy"); err != nil {
		t.Fatalf("ReconnectServer() error = %v", err)
	}

	after, ok := r.GetEntry("proxy")
	if !ok {
		t.Fatal("GetEntry(proxy) = !ok, want ok")
	}
	if after.Status != StatusConnected {
		t.Fatalf("after status = %q, want connected", after.Status)
	}
	if len(after.Tools) != 1 || after.Tools[0].Name != "tool_one" {
		t.Fatalf("after tools = %#v", after.Tools)
	}
}

func TestServerRegistryConnectAllHTTP(t *testing.T) {
	t.Parallel()

	server := newRegistryHTTPServer(t)
	defer server.Close()

	r := NewServerRegistry()
	r.LoadConfigs(map[string]client.ServerConfig{
		"http": {Type: "http", URL: server.URL, Headers: map[string]string{"X-Test": "ok"}},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	r.ConnectAll(ctx)

	entries := r.List()
	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}
	if entries[0].Status != StatusConnected {
		t.Fatalf("status = %q, want connected", entries[0].Status)
	}
	if len(entries[0].Tools) != 1 || entries[0].Tools[0].Name != "tool_one" {
		t.Fatalf("tools = %#v", entries[0].Tools)
	}
	if len(entries[0].Resources) != 1 || entries[0].Resources[0].URI != "file:///tmp/a" {
		t.Fatalf("resources = %#v", entries[0].Resources)
	}
	if len(entries[0].Prompts) != 1 || entries[0].Prompts[0].Name != "summarize" {
		t.Fatalf("prompts = %#v", entries[0].Prompts)
	}
}

func TestServerRegistryConnectAllSSE(t *testing.T) {
	t.Parallel()

	server := newRegistryRemoteSSEServer(t)
	defer server.Close()

	r := NewServerRegistry()
	r.LoadConfigs(map[string]client.ServerConfig{
		"sse": {Type: "sse", URL: server.URL},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	r.ConnectAll(ctx)

	entries := r.List()
	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}
	if entries[0].Status != StatusConnected {
		t.Fatalf("status = %q, want connected", entries[0].Status)
	}
	if len(entries[0].Tools) != 1 || entries[0].Tools[0].Name != "tool_one" {
		t.Fatalf("tools = %#v", entries[0].Tools)
	}
}

func TestServerRegistryConnectAllWebSocket(t *testing.T) {
	t.Parallel()

	server := newRegistryRemoteWebSocketServer(t)
	defer server.Close()

	wsURL := strings.Replace(server.URL, "http://", "ws://", 1)
	r := NewServerRegistry()
	r.LoadConfigs(map[string]client.ServerConfig{
		"ws": {Type: "ws", URL: wsURL},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	r.ConnectAll(ctx)

	entries := r.List()
	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}
	if entries[0].Status != StatusConnected {
		t.Fatalf("status = %q, want connected", entries[0].Status)
	}
	if len(entries[0].Tools) != 1 || entries[0].Tools[0].Name != "tool_one" {
		t.Fatalf("tools = %#v", entries[0].Tools)
	}
}

func newRegistryHTTPServer(t *testing.T) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if got := r.Header.Get("X-Test"); got != "ok" {
			t.Fatalf("X-Test header = %q, want ok", got)
		}

		var req client.JSONRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}

		resp := client.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID}
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
			resp.Error = &client.JSONRPCError{
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

func newRegistryClaudeAiProxyServer(t *testing.T, wantAuth string) *httptest.Server {
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
		if got := r.Header.Get("Authorization"); got != "Bearer "+wantAuth {
			t.Fatalf("Authorization header = %q, want Bearer %s", got, wantAuth)
		}

		var req client.JSONRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}

		resp := client.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID}
		switch req.Method {
		case "initialize":
			resp.Result = json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{"tools":{"listChanged":false},"resources":{"listChanged":false},"prompts":{"listChanged":false}},"serverInfo":{"name":"claudeai-proxy-test","version":"1.0"}}`)
		case "tools/list":
			resp.Result = json.RawMessage(`{"tools":[{"name":"tool_one","description":"one"}]}`)
		case "resources/list":
			resp.Result = json.RawMessage(`{"resources":[{"uri":"file:///tmp/a","name":"config"}]}`)
		case "prompts/list":
			resp.Result = json.RawMessage(`{"prompts":[{"name":"summarize","description":"Summarize"}]}`)
		default:
			resp.Error = &client.JSONRPCError{
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

func TestServerRegistryCloseAll(t *testing.T) {
	r := NewServerRegistry()
	// CloseAll on empty registry should not panic.
	r.CloseAll()
}

func TestServerRegistryConnected(t *testing.T) {
	r := NewServerRegistry()
	r.LoadConfigs(map[string]client.ServerConfig{
		"a": {Command: "echo"},
		"b": {Command: "echo"},
	})
	// Manually set one entry to connected for the filter test.
	for i := range r.entries {
		if r.entries[i].Name == "a" {
			r.entries[i].Status = StatusConnected
		}
	}

	connected := r.Connected()
	if len(connected) != 1 {
		t.Fatalf("len(connected) = %d, want 1", len(connected))
	}
	if connected[0].Name != "a" {
		t.Fatalf("connected[0].name = %q", connected[0].Name)
	}
}

func TestServerRegistryStoresCapabilities(t *testing.T) {
	r := NewServerRegistry()
	r.LoadConfigs(map[string]client.ServerConfig{
		"caps": {
			Command: "sh",
			Args: []string{"-c", `
				while IFS= read -r line; do
					id=$(printf '%s' "$line" | sed -n 's/.*"id":"\([^"]*\)".*/\1/p')
					method=$(printf '%s' "$line" | sed -n 's/.*"method":"\([^"]*\)".*/\1/p')
					case "$method" in
						initialize)
							printf '%s\n' '{"jsonrpc":"2.0","id":"'"$id"'","result":{"protocolVersion":"2024-11-05","capabilities":{"tools":{},"resources":{},"prompts":{}},"serverInfo":{"name":"caps","version":"1.0"}}}'
							;;
						tools/list)
							printf '%s\n' '{"jsonrpc":"2.0","id":"'"$id"'","result":{"tools":[{"name":"tool_one","description":"Tool one"}]}}'
							;;
						resources/list)
							printf '%s\n' '{"jsonrpc":"2.0","id":"'"$id"'","result":{"resources":[{"uri":"file:///tmp/a","name":"config","description":"Config file"}]}}'
							;;
						prompts/list)
							printf '%s\n' '{"jsonrpc":"2.0","id":"'"$id"'","result":{"prompts":[{"name":"summarize","description":"Summarize"}]}}'
							;;
					esac
				done
			`},
		},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	r.ConnectAll(ctx)

	entries := r.List()
	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}
	if entries[0].Capabilities.Tools == nil || entries[0].Capabilities.Resources == nil || entries[0].Capabilities.Prompts == nil {
		t.Fatalf("capabilities = %#v", entries[0].Capabilities)
	}
	if len(entries[0].Tools) != 1 || entries[0].Tools[0].Name != "tool_one" {
		t.Fatalf("tools = %#v", entries[0].Tools)
	}
	if len(entries[0].Resources) != 1 || entries[0].Resources[0].URI != "file:///tmp/a" {
		t.Fatalf("resources = %#v", entries[0].Resources)
	}
	if len(entries[0].Prompts) != 1 || entries[0].Prompts[0].Name != "summarize" {
		t.Fatalf("prompts = %#v", entries[0].Prompts)
	}
}

func TestServerRegistryRefreshesToolsSnapshotOnNotification(t *testing.T) {
	r := NewServerRegistry()
	r.LoadConfigs(map[string]client.ServerConfig{
		"caps": {
			Command: "sh",
			Args: []string{"-c", `
				tools_count=0
				while IFS= read -r line; do
					id=$(printf '%s' "$line" | sed -n 's/.*"id":"\([^"]*\)".*/\1/p')
					method=$(printf '%s' "$line" | sed -n 's/.*"method":"\([^"]*\)".*/\1/p')
					case "$method" in
						initialize)
							printf '%s\n' '{"jsonrpc":"2.0","id":"'"$id"'","result":{"protocolVersion":"2024-11-05","capabilities":{"tools":{"listChanged":true},"resources":{},"prompts":{}},"serverInfo":{"name":"caps","version":"1.0"}}}'
							(sleep 0.2; printf '%s\n' '{"jsonrpc":"2.0","method":"tools/list_changed"}') &
							;;
						tools/list)
							tools_count=$((tools_count + 1))
							if [ "$tools_count" -eq 1 ]; then
								printf '%s\n' '{"jsonrpc":"2.0","id":"'"$id"'","result":{"tools":[{"name":"tool_one"}]}}'
							else
								printf '%s\n' '{"jsonrpc":"2.0","id":"'"$id"'","result":{"tools":[{"name":"tool_two"}]}}'
							fi
							;;
						resources/list)
							printf '%s\n' '{"jsonrpc":"2.0","id":"'"$id"'","result":{"resources":[{"uri":"file:///tmp/a","name":"config"}]}}'
							;;
						prompts/list)
							printf '%s\n' '{"jsonrpc":"2.0","id":"'"$id"'","result":{"prompts":[{"name":"summarize"}]}}'
							;;
					esac
				done
			`},
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	r.ConnectAll(ctx)

	waitForEntryTools := func(want string) bool {
		for _, entry := range r.List() {
			if entry.Name != "caps" {
				continue
			}
			return len(entry.Tools) == 1 && entry.Tools[0].Name == want
		}
		return false
	}

	deadline := time.After(5 * time.Second)
	for {
		if waitForEntryTools("tool_two") {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for refreshed tools snapshot: %#v", r.List())
		default:
			time.Sleep(50 * time.Millisecond)
		}
	}
}

func TestSetGetLastRegistry(t *testing.T) {
	r := NewServerRegistry()
	SetLastRegistry(r)
	if GetLastRegistry() != r {
		t.Fatal("GetLastRegistry did not return the set registry")
	}
}

func TestServerRegistryGetEntry(t *testing.T) {
	r := NewServerRegistry()
	r.LoadConfigs(map[string]client.ServerConfig{
		"a": {Command: "echo"},
		"b": {Command: "cat"},
	})

	entry, ok := r.GetEntry("a")
	if !ok {
		t.Fatal("expected to find entry 'a'")
	}
	if entry.Name != "a" {
		t.Fatalf("name = %q, want 'a'", entry.Name)
	}

	_, ok = r.GetEntry("missing")
	if ok {
		t.Fatal("expected not to find 'missing'")
	}
}

func TestServerRegistryConnectDynamicServer(t *testing.T) {
	server := newRegistryHTTPServer(t)
	defer server.Close()

	r := NewServerRegistry()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	entry, err := r.ConnectDynamicServer(ctx, "dynamic-http", client.ServerConfig{
		Type:    "http",
		URL:     server.URL,
		Headers: map[string]string{"X-Test": "ok"},
	})
	if err != nil {
		t.Fatalf("ConnectDynamicServer failed: %v", err)
	}
	if entry.Status != StatusConnected {
		t.Fatalf("status = %q, want connected", entry.Status)
	}
	if len(entry.Tools) != 1 || entry.Tools[0].Name != "tool_one" {
		t.Fatalf("tools = %#v", entry.Tools)
	}

	// Verify the entry is stored in the registry.
	stored, ok := r.GetEntry("dynamic-http")
	if !ok {
		t.Fatal("dynamic-http not found in registry after connect")
	}
	if stored.Status != StatusConnected {
		t.Fatalf("stored status = %q, want connected", stored.Status)
	}
}

func TestServerRegistryDisconnectServer(t *testing.T) {
	server := newRegistryHTTPServer(t)
	defer server.Close()

	r := NewServerRegistry()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := r.ConnectDynamicServer(ctx, "to-remove", client.ServerConfig{
		Type:    "http",
		URL:     server.URL,
		Headers: map[string]string{"X-Test": "ok"},
	})
	if err != nil {
		t.Fatalf("ConnectDynamicServer failed: %v", err)
	}

	if _, ok := r.GetEntry("to-remove"); !ok {
		t.Fatal("expected entry before disconnect")
	}

	if err := r.DisconnectServer("to-remove"); err != nil {
		t.Fatalf("DisconnectServer failed: %v", err)
	}

	if _, ok := r.GetEntry("to-remove"); ok {
		t.Fatal("expected entry to be removed after disconnect")
	}
}

func TestServerRegistryDisconnectServer_NotFound(t *testing.T) {
	r := NewServerRegistry()
	err := r.DisconnectServer("missing")
	if err == nil {
		t.Fatal("expected error for missing server")
	}
}

func newRegistryRemoteSSEServer(t *testing.T) *httptest.Server {
	t.Helper()

	events := make(chan []byte, 16)
	serveResponse := func(req client.JSONRPCRequest) client.JSONRPCResponse {
		switch req.Method {
		case "initialize":
			return client.JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{"tools":{"listChanged":false},"resources":{"listChanged":false},"prompts":{"listChanged":false}},"serverInfo":{"name":"sse-test","version":"1.0"}}`),
			}
		case "tools/list":
			return client.JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  json.RawMessage(`{"tools":[{"name":"tool_one","description":"one"}]}`),
			}
		case "resources/list":
			return client.JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  json.RawMessage(`{"resources":[{"uri":"file:///tmp/a","name":"config"}]}`),
			}
		case "prompts/list":
			return client.JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  json.RawMessage(`{"prompts":[{"name":"summarize","description":"Summarize"}]}`),
			}
		default:
			return client.JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error: &client.JSONRPCError{
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
			var req client.JSONRPCRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			payload, err := json.Marshal(serveResponse(req))
			if err != nil {
				t.Fatalf("marshal response: %v", err)
			}
			events <- payload
			w.WriteHeader(http.StatusAccepted)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))

	return server
}

func newRegistryRemoteWebSocketServer(t *testing.T) *httptest.Server {
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
			var req client.JSONRPCRequest
			if err := json.Unmarshal(msg, &req); err != nil {
				t.Fatalf("unmarshal request: %v", err)
			}
			resp := client.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID}
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
				resp.Error = &client.JSONRPCError{Code: -32601, Message: fmt.Sprintf("unknown method %q", req.Method)}
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
