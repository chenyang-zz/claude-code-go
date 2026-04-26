package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/agent"
	"github.com/sheepzhao/claude-code-go/internal/core/model"
	mcpclient "github.com/sheepzhao/claude-code-go/internal/platform/mcp/client"
	mcpregistry "github.com/sheepzhao/claude-code-go/internal/platform/mcp/registry"
)

func TestMapAgentMCPServerSpecToConfig(t *testing.T) {
	tests := []struct {
		name    string
		spec    agent.AgentMCPServerSpec
		want    mcpclient.ServerConfig
		wantErr bool
	}{
		{
			name: "full inline config",
			spec: agent.AgentMCPServerSpec{
				Name: "test-server",
				Config: map[string]any{
					"type":          "stdio",
					"command":       "npx",
					"args":          []any{"-y", "@modelcontextprotocol/server-filesystem"},
					"env":           map[string]any{"KEY": "value"},
					"url":           "https://example.com",
					"headers":       map[string]any{"Authorization": "Bearer token"},
					"headersHelper": "./get-headers.sh",
				},
			},
			want: mcpclient.ServerConfig{
				Type:          "stdio",
				Command:       "npx",
				Args:          []string{"-y", "@modelcontextprotocol/server-filesystem"},
				Env:           map[string]string{"KEY": "value"},
				URL:           "https://example.com",
				Headers:       map[string]string{"Authorization": "Bearer token"},
				HeadersHelper: "./get-headers.sh",
			},
		},
		{
			name: "minimal inline config",
			spec: agent.AgentMCPServerSpec{
				Name:   "minimal",
				Config: map[string]any{"command": "echo"},
			},
			want: mcpclient.ServerConfig{
				Command: "echo",
			},
		},
		{
			name: "empty inline config",
			spec: agent.AgentMCPServerSpec{
				Name:   "empty",
				Config: map[string]any{},
			},
			want: mcpclient.ServerConfig{},
		},
		{
			name:    "nil config errors",
			spec:    agent.AgentMCPServerSpec{Name: "ref", Config: nil},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := mapAgentMCPServerSpecToConfig(tt.spec)
			if (err != nil) != tt.wantErr {
				t.Fatalf("mapAgentMCPServerSpecToConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if got.Type != tt.want.Type {
				t.Errorf("Type = %q, want %q", got.Type, tt.want.Type)
			}
			if got.Command != tt.want.Command {
				t.Errorf("Command = %q, want %q", got.Command, tt.want.Command)
			}
			if len(got.Args) != len(tt.want.Args) {
				t.Errorf("Args = %v, want %v", got.Args, tt.want.Args)
			}
			if len(got.Env) != len(tt.want.Env) {
				t.Errorf("Env len = %d, want %d", len(got.Env), len(tt.want.Env))
			}
			if got.URL != tt.want.URL {
				t.Errorf("URL = %q, want %q", got.URL, tt.want.URL)
			}
			if len(got.Headers) != len(tt.want.Headers) {
				t.Errorf("Headers len = %d, want %d", len(got.Headers), len(tt.want.Headers))
			}
			if got.HeadersHelper != tt.want.HeadersHelper {
				t.Errorf("HeadersHelper = %q, want %q", got.HeadersHelper, tt.want.HeadersHelper)
			}
		})
	}
}

func TestMergeToolDefinitions(t *testing.T) {
	tests := []struct {
		name    string
		base    []model.ToolDefinition
		overlay []model.ToolDefinition
		want    []string
	}{
		{
			name:    "both empty",
			base:    nil,
			overlay: nil,
			want:    nil,
		},
		{
			name:    "only base",
			base:    []model.ToolDefinition{{Name: "a"}, {Name: "b"}},
			overlay: nil,
			want:    []string{"a", "b"},
		},
		{
			name:    "only overlay",
			base:    nil,
			overlay: []model.ToolDefinition{{Name: "c"}},
			want:    []string{"c"},
		},
		{
			name:    "no overlap",
			base:    []model.ToolDefinition{{Name: "a"}},
			overlay: []model.ToolDefinition{{Name: "b"}},
			want:    []string{"a", "b"},
		},
		{
			name:    "base takes precedence",
			base:    []model.ToolDefinition{{Name: "a", Description: "base-a"}},
			overlay: []model.ToolDefinition{{Name: "a", Description: "overlay-a"}, {Name: "b"}},
			want:    []string{"a", "b"},
		},
		{
			name:    "deduplication within base",
			base:    []model.ToolDefinition{{Name: "a"}, {Name: "a"}},
			overlay: nil,
			want:    []string{"a"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mergeToolDefinitions(tt.base, tt.overlay)
			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d; got %v", len(got), len(tt.want), namesFromDefs(got))
			}
			for i, wantName := range tt.want {
				if got[i].Name != wantName {
					t.Errorf("got[%d].Name = %q, want %q", i, got[i].Name, wantName)
				}
			}
		})
	}
}

func namesFromDefs(defs []model.ToolDefinition) []string {
	out := make([]string, len(defs))
	for i, d := range defs {
		out[i] = d.Name
	}
	return out
}

func TestInitializeAgentMCPServers_Empty(t *testing.T) {
	r := &Runner{}
	result, err := r.initializeAgentMCPServers(context.Background(), agent.Definition{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.toolDefs) != 0 {
		t.Fatalf("expected no tool defs, got %d", len(result.toolDefs))
	}
	if len(result.tools) != 0 {
		t.Fatalf("expected no tools, got %d", len(result.tools))
	}
	// Cleanup should be a no-op.
	result.cleanup()
}

func TestInitializeAgentMCPServers_NoRegistry(t *testing.T) {
	r := &Runner{}
	def := agent.Definition{
		MCPServers: []agent.AgentMCPServerSpec{
			{Name: "missing", Config: nil},
		},
	}
	result, err := r.initializeAgentMCPServers(context.Background(), def)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.toolDefs) != 0 {
		t.Fatalf("expected no tool defs when registry is nil, got %d", len(result.toolDefs))
	}
}

func TestInitializeAgentMCPServers_NameReferenceNotFound(t *testing.T) {
	reg := mcpregistry.NewServerRegistry()
	r := &Runner{ServerRegistry: reg}
	def := agent.Definition{
		MCPServers: []agent.AgentMCPServerSpec{
			{Name: "nonexistent", Config: nil},
		},
	}
	result, err := r.initializeAgentMCPServers(context.Background(), def)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.toolDefs) != 0 {
		t.Fatalf("expected no tool defs for missing reference, got %d", len(result.toolDefs))
	}
}

func TestInitializeAgentMCPServers_NameReferenceFound(t *testing.T) {
	server := newRegistryHTTPServer(t)
	defer server.Close()

	reg := mcpregistry.NewServerRegistry()
	ctx, cancel := context.WithTimeout(context.Background(), 5*1000000000)
	defer cancel()
	_, err := reg.ConnectDynamicServer(ctx, "existing", mcpclient.ServerConfig{
		Type: "http",
		URL:  server.URL,
	})
	if err != nil {
		t.Fatalf("setup ConnectDynamicServer failed: %v", err)
	}

	r := &Runner{ServerRegistry: reg}
	def := agent.Definition{
		AgentType: "test-agent",
		MCPServers: []agent.AgentMCPServerSpec{
			{Name: "existing", Config: nil},
		},
	}
	result, err := r.initializeAgentMCPServers(context.Background(), def)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.toolDefs) != 1 {
		t.Fatalf("expected 1 tool def, got %d", len(result.toolDefs))
	}
	if result.toolDefs[0].Name != "existing__tool_one" {
		t.Fatalf("tool name = %q, want existing__tool_one", result.toolDefs[0].Name)
	}
	// Cleanup for name references should be a no-op (no newly created servers).
	result.cleanup()

	// Ensure the entry is still present after cleanup.
	if _, ok := reg.GetEntry("existing"); !ok {
		t.Fatal("name-reference entry should not be cleaned up")
	}
}

func TestInitializeAgentMCPServers_InlineDefinition(t *testing.T) {
	server := newRegistryHTTPServer(t)
	defer server.Close()

	reg := mcpregistry.NewServerRegistry()
	r := &Runner{ServerRegistry: reg}
	def := agent.Definition{
		AgentType: "test-agent",
		MCPServers: []agent.AgentMCPServerSpec{
			{
				Name: "inline-server",
				Config: map[string]any{
					"type": "http",
					"url":  server.URL,
				},
			},
		},
	}
	result, err := r.initializeAgentMCPServers(context.Background(), def)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.toolDefs) != 1 {
		t.Fatalf("expected 1 tool def, got %d", len(result.toolDefs))
	}
	if result.toolDefs[0].Name != "inline-server__tool_one" {
		t.Fatalf("tool name = %q, want inline-server__tool_one", result.toolDefs[0].Name)
	}

	// Inline servers should be cleaned up.
	result.cleanup()
	if _, ok := reg.GetEntry("inline-server"); ok {
		t.Fatal("inline entry should be cleaned up after cleanup")
	}
}

func TestInitializeAgentMCPServers_MixedNameAndInline(t *testing.T) {
	server := newRegistryHTTPServer(t)
	defer server.Close()

	reg := mcpregistry.NewServerRegistry()
	ctx, cancel := context.WithTimeout(context.Background(), 5*1000000000)
	defer cancel()
	_, err := reg.ConnectDynamicServer(ctx, "global", mcpclient.ServerConfig{
		Type: "http",
		URL:  server.URL,
	})
	if err != nil {
		t.Fatalf("setup ConnectDynamicServer failed: %v", err)
	}

	r := &Runner{ServerRegistry: reg}
	def := agent.Definition{
		AgentType: "test-agent",
		MCPServers: []agent.AgentMCPServerSpec{
			{Name: "global", Config: nil}, // name reference
			{
				Name: "inline",
				Config: map[string]any{
					"type": "http",
					"url":  server.URL,
				},
			},
		},
	}
	result, err := r.initializeAgentMCPServers(context.Background(), def)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.toolDefs) != 2 {
		t.Fatalf("expected 2 tool defs, got %d", len(result.toolDefs))
	}

	result.cleanup()
	// Global entry should survive.
	if _, ok := reg.GetEntry("global"); !ok {
		t.Fatal("global entry should survive cleanup")
	}
	// Inline entry should be removed.
	if _, ok := reg.GetEntry("inline"); ok {
		t.Fatal("inline entry should be removed after cleanup")
	}
}

// newRegistryHTTPServer creates a minimal HTTP MCP server for testing.
// It is duplicated here to avoid import cycles; the original lives in the registry package tests.
func newRegistryHTTPServer(t *testing.T) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var req mcpclient.JSONRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}

		resp := mcpclient.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID}
		switch req.Method {
		case "initialize":
			resp.Result = json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{"tools":{"listChanged":false},"resources":{"listChanged":false},"prompts":{"listChanged":false}},"serverInfo":{"name":"http-test","version":"1.0"}}`)
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
