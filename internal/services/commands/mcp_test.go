package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	mcpclient "github.com/sheepzhao/claude-code-go/internal/platform/mcp/client"
	mcpregistry "github.com/sheepzhao/claude-code-go/internal/platform/mcp/registry"
)

func newCommandClaudeAiProxyServer(t *testing.T, wantAuth string) *httptest.Server {
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

		var req mcpclient.JSONRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}

		resp := mcpclient.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID}
		switch req.Method {
		case "initialize":
			resp.Result = json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{"tools":{"listChanged":false}},"serverInfo":{"name":"proxy-test","version":"1.0"}}`)
		case "tools/list":
			resp.Result = json.RawMessage(`{"tools":[{"name":"tool_one","description":"one"}]}`)
		default:
			resp.Error = &mcpclient.JSONRPCError{
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

// TestMCPCommandMetadata verifies /mcp exposes stable metadata.
func TestMCPCommandMetadata(t *testing.T) {
	meta := MCPCommand{}.Metadata()
	if meta.Name != "mcp" {
		t.Fatalf("Metadata().Name = %q, want mcp", meta.Name)
	}
	if meta.Description != "Manage MCP servers" {
		t.Fatalf("Metadata().Description = %q, want stable mcp description", meta.Description)
	}
	if !strings.Contains(meta.Usage, "detail") {
		t.Fatalf("Metadata().Usage = %q, should contain detail subcommand", meta.Usage)
	}
	if !strings.Contains(meta.Usage, "authenticate") {
		t.Fatalf("Metadata().Usage = %q, should contain authenticate subcommand", meta.Usage)
	}
}

// TestMCPCommandExecuteReportsFallback verifies /mcp reports the current Go host fallback before MCP management is migrated.
func TestMCPCommandExecuteReportsFallback(t *testing.T) {
	result, err := MCPCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	want := "No MCP servers configured. Set CLAUDE_CODE_MCP_SERVERS to configure servers."
	if result.Output != want {
		t.Fatalf("Execute() output = %q, want %q", result.Output, want)
	}
}

// TestMCPCommandExecuteDetailNotFound verifies /mcp detail returns an error for unknown servers.
func TestMCPCommandExecuteDetailNotFound(t *testing.T) {
	reg := mcpregistry.NewServerRegistry()
	mcpregistry.SetLastRegistry(reg)
	defer mcpregistry.SetLastRegistry(nil)

	cmd := MCPCommand{}
	result, err := cmd.Execute(context.Background(), command.Args{Raw: []string{"detail", "nonexistent"}})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(result.Output, "not found") {
		t.Fatalf("expected 'not found' in output, got: %q", result.Output)
	}
}

// TestMCPCommandExecuteDetailEmptyRegistry verifies /mcp detail with empty registry.
func TestMCPCommandExecuteDetailEmptyRegistry(t *testing.T) {
	reg := mcpregistry.NewServerRegistry()
	mcpregistry.SetLastRegistry(reg)
	defer mcpregistry.SetLastRegistry(nil)

	cmd := MCPCommand{}
	result, err := cmd.Execute(context.Background(), command.Args{Raw: []string{"detail", "missing"}})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(result.Output, "not found") {
		t.Fatalf("expected 'not found' in output, got: %q", result.Output)
	}
}

// TestMCPCommandExecuteAuthenticateRejectsInvalidArgs verifies /mcp authenticate enforces arity.
func TestMCPCommandExecuteAuthenticateRejectsInvalidArgs(t *testing.T) {
	cmd := MCPCommand{}
	_, err := cmd.Execute(context.Background(), command.Args{Raw: []string{"authenticate"}})
	if err == nil {
		t.Fatal("Execute() error = nil, want usage error")
	}
	if err.Error() != "usage: /mcp authenticate <server-name>" {
		t.Fatalf("Execute() error = %q, want usage error", err.Error())
	}
}

// TestMCPCommandExecuteAuthenticateNotFound verifies /mcp authenticate reports unknown server.
func TestMCPCommandExecuteAuthenticateNotFound(t *testing.T) {
	reg := mcpregistry.NewServerRegistry()
	mcpregistry.SetLastRegistry(reg)
	defer mcpregistry.SetLastRegistry(nil)

	cmd := MCPCommand{}
	result, err := cmd.Execute(context.Background(), command.Args{Raw: []string{"authenticate", "nonexistent"}})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(result.Output, "not found") {
		t.Fatalf("output = %q, want not found", result.Output)
	}
}

// TestMCPCommandExecuteAuthenticateReconnectsClaudeAiProxy verifies /mcp authenticate can reconnect a needs-auth claudeai-proxy server after token injection.
func TestMCPCommandExecuteAuthenticateReconnectsClaudeAiProxy(t *testing.T) {
	server := newCommandClaudeAiProxyServer(t, "token-123")
	defer server.Close()

	reg := mcpregistry.NewServerRegistry()
	reg.LoadConfigs(map[string]mcpclient.ServerConfig{
		"proxy": {Type: "claudeai-proxy", URL: server.URL, Headers: map[string]string{"X-Test": "ok"}},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	reg.ConnectAll(ctx)

	before, ok := reg.GetEntry("proxy")
	if !ok || before.Status != mcpregistry.StatusNeedsAuth {
		t.Fatalf("before status = %#v, want needs-auth", before.Status)
	}

	reg.SetAuthToken("token-123")
	mcpregistry.SetLastRegistry(reg)
	defer mcpregistry.SetLastRegistry(nil)

	cmd := MCPCommand{}
	result, err := cmd.Execute(context.Background(), command.Args{Raw: []string{"authenticate", "proxy"}})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(result.Output, "reconnected successfully") {
		t.Fatalf("output = %q, want reconnect success", result.Output)
	}
	after, ok := reg.GetEntry("proxy")
	if !ok || after.Status != mcpregistry.StatusConnected {
		t.Fatalf("after status = %#v, want connected", after.Status)
	}
}

// TestMCPCommandExecuteAuthenticateOAuthUnsupported verifies /mcp authenticate reports OAuth interactive flow as unmigrated.
func TestMCPCommandExecuteAuthenticateOAuthUnsupported(t *testing.T) {
	reg := mcpregistry.NewServerRegistry()
	reg.LoadConfigs(map[string]mcpclient.ServerConfig{
		"oauth-srv": {
			Type: "http",
			URL:  "https://example.invalid/mcp",
			OAuth: &mcpclient.OAuthConfig{
				ClientID: "client-1",
			},
		},
	})
	mcpregistry.SetLastRegistry(reg)
	defer mcpregistry.SetLastRegistry(nil)

	cmd := MCPCommand{}
	result, err := cmd.Execute(context.Background(), command.Args{Raw: []string{"authenticate", "oauth-srv"}})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(result.Output, "not implemented") {
		t.Fatalf("output = %q, want OAuth unsupported hint", result.Output)
	}
}

// TestMCPCommandExecuteNeedsAuthShowsNextAction verifies /mcp list shows authenticate next action for recoverable servers.
func TestMCPCommandExecuteNeedsAuthShowsNextAction(t *testing.T) {
	reg := mcpregistry.NewServerRegistry()
	reg.LoadConfigs(map[string]mcpclient.ServerConfig{
		"proxy": {Type: "claudeai-proxy", URL: "https://example.invalid/mcp"},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	reg.ConnectAll(ctx)
	mcpregistry.SetLastRegistry(reg)
	defer mcpregistry.SetLastRegistry(nil)

	cmd := MCPCommand{}
	result, err := cmd.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(result.Output, "next: /mcp authenticate proxy") {
		t.Fatalf("output = %q, want authenticate next action", result.Output)
	}
}

// TestMCPCommandExecuteDetailNeedsAuthShowsNextAction verifies /mcp detail shows the next action for needs-auth server.
func TestMCPCommandExecuteDetailNeedsAuthShowsNextAction(t *testing.T) {
	reg := mcpregistry.NewServerRegistry()
	reg.LoadConfigs(map[string]mcpclient.ServerConfig{
		"proxy": {Type: "claudeai-proxy", URL: "https://example.invalid/mcp"},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	reg.ConnectAll(ctx)
	mcpregistry.SetLastRegistry(reg)
	defer mcpregistry.SetLastRegistry(nil)

	cmd := MCPCommand{}
	result, err := cmd.Execute(context.Background(), command.Args{Raw: []string{"detail", "proxy"}})
	if err != nil {
		t.Fatalf("Execute(detail) error = %v", err)
	}
	if !strings.Contains(result.Output, "Next action: run /mcp authenticate proxy") {
		t.Fatalf("detail output = %q, want next action", result.Output)
	}
}

// TestMCPCommandExecuteDetailShowsOAuthConfig verifies /mcp detail surfaces oauth metadata when present.
func TestMCPCommandExecuteDetailShowsOAuthConfig(t *testing.T) {
	reg := mcpregistry.NewServerRegistry()
	reg.LoadConfigs(map[string]mcpclient.ServerConfig{
		"proxy": {
			Type: "http",
			URL:  "https://example.invalid/mcp",
			OAuth: &mcpclient.OAuthConfig{
				ClientID:              "client-123",
				AuthServerMetadataURL: "https://auth.example.invalid/.well-known/oauth-authorization-server",
				XAA:                   boolPtr(true),
			},
		},
	})
	mcpregistry.SetLastRegistry(reg)
	defer mcpregistry.SetLastRegistry(nil)

	cmd := MCPCommand{}
	result, err := cmd.Execute(context.Background(), command.Args{Raw: []string{"detail", "proxy"}})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	for _, want := range []string{"OAuth: configured", "XAA: enabled", "Server is not connected"} {
		if !strings.Contains(result.Output, want) {
			t.Fatalf("detail output = %q, want %q", result.Output, want)
		}
	}
}

// TestMCPCommandExecuteListEmptyRegistry verifies /mcp with an empty registry.
func TestMCPCommandExecuteListEmptyRegistry(t *testing.T) {
	reg := mcpregistry.NewServerRegistry()
	mcpregistry.SetLastRegistry(reg)
	defer mcpregistry.SetLastRegistry(nil)

	cmd := MCPCommand{}
	result, err := cmd.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Output != "No MCP servers configured." {
		t.Fatalf("Execute() output = %q, want 'No MCP servers configured.'", result.Output)
	}
}

func boolPtr(v bool) *bool {
	return &v
}

func TestMCPCommandExecuteShowsResourcesAndPrompts(t *testing.T) {
	reg := mcpregistry.NewServerRegistry()
	reg.LoadConfigs(map[string]mcpclient.ServerConfig{
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
							printf '%s\n' '{"jsonrpc":"2.0","id":"'"$id"'","result":{"prompts":[{"name":"summarize","description":"Summarize","arguments":{"path":{"name":"path","description":"Target file","required":true}}}]}}'
							;;
					esac
				done
			`},
		},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	reg.ConnectAll(ctx)
	mcpregistry.SetLastRegistry(reg)
	defer mcpregistry.SetLastRegistry(nil)

	cmd := MCPCommand{}
	result, err := cmd.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	for _, want := range []string{"1 tools", "1 resources", "1 prompts"} {
		if !strings.Contains(result.Output, want) {
			t.Fatalf("Execute() output = %q, want %q", result.Output, want)
		}
	}

	detail, err := cmd.Execute(context.Background(), command.Args{Raw: []string{"detail", "caps"}})
	if err != nil {
		t.Fatalf("Execute(detail) error = %v", err)
	}
	for _, want := range []string{"Tools (1):", "Resources (1):", "Prompts (1):", "file:///tmp/a", "summarize"} {
		if !strings.Contains(detail.Output, want) {
			t.Fatalf("detail output = %q, want %q", detail.Output, want)
		}
	}
	if reflect.DeepEqual(detail.Output, "") {
		t.Fatal("detail output should not be empty")
	}
}
