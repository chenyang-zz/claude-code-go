package list_resources

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/tool"
	mcpclient "github.com/sheepzhao/claude-code-go/internal/platform/mcp/client"
	mcpregistry "github.com/sheepzhao/claude-code-go/internal/platform/mcp/registry"
)

// stubTransport implements client.Transport for testing without a real MCP server.
type stubTransport struct{}

func (s *stubTransport) Send(ctx context.Context, req mcpclient.JSONRPCRequest) (*mcpclient.JSONRPCResponse, error) {
	return &mcpclient.JSONRPCResponse{}, nil
}
func (s *stubTransport) Close() error { return nil }
func (s *stubTransport) SetNotificationHandler(method string, handler mcpclient.NotificationHandler) {
}
func (s *stubTransport) SetRequestHandler(method string, handler mcpclient.RequestHandler) {}

// stubRegistry creates a test ServerRegistry and seeds it with entries.
func stubRegistry() *mcpregistry.ServerRegistry {
	r := mcpregistry.NewServerRegistry()
	r.LoadConfigs(map[string]mcpclient.ServerConfig{
		"server-a": {Command: "echo", Args: []string{"a"}},
		"server-b": {Command: "echo", Args: []string{"b"}},
	})
	return r
}

func TestListResources_Name(t *testing.T) {
	tr := NewTool()
	if tr.Name() != Name {
		t.Fatalf("expected Name %q, got %q", Name, tr.Name())
	}
}

func TestListResources_Description(t *testing.T) {
	tr := NewTool()
	if tr.Description() == "" {
		t.Fatalf("expected non-empty Description")
	}
}

func TestListResources_InputSchema(t *testing.T) {
	tr := NewTool()
	schema := tr.InputSchema()
	if _, ok := schema.Properties["server"]; !ok {
		t.Fatalf("expected 'server' property in input schema")
	}
}

func TestListResources_IsReadOnly(t *testing.T) {
	tr := NewTool()
	if !tr.IsReadOnly() {
		t.Fatalf("expected IsReadOnly to return true")
	}
}

func TestListResources_IsConcurrencySafe(t *testing.T) {
	tr := NewTool()
	if !tr.IsConcurrencySafe() {
		t.Fatalf("expected IsConcurrencySafe to return true")
	}
}

func TestListResources_Invoke_RegistryNil(t *testing.T) {
	mcpregistry.SetLastRegistry(nil)

	tr := NewTool()
	result, err := tr.Invoke(context.Background(), tool.Call{Input: map[string]any{}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected tool error: %s", result.Error)
	}
	if result.Output != "[]" {
		t.Fatalf("expected empty array output for nil registry, got %q", result.Output)
	}
}

func TestListResources_Invoke_NoResources(t *testing.T) {
	r := stubRegistry()
	mcpregistry.SetLastRegistry(r)

	tr := NewTool()
	result, err := tr.Invoke(context.Background(), tool.Call{Input: map[string]any{}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected tool error: %s", result.Error)
	}

	var resources []resourceEntry
	if err := json.Unmarshal([]byte(result.Output), &resources); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}
	if len(resources) != 0 {
		t.Fatalf("expected 0 resources, got %d", len(resources))
	}
}

func TestListResources_Invoke_SingleServer(t *testing.T) {
	r := stubRegistry()
	mcpregistry.SetLastRegistry(r)

	tr := NewTool()
	result, err := tr.Invoke(context.Background(), tool.Call{
		Input: map[string]any{"server": "server-a"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected tool error: %s", result.Error)
	}

	var resources []resourceEntry
	if err := json.Unmarshal([]byte(result.Output), &resources); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}
	if len(resources) != 0 {
		t.Fatalf("expected 0 resources for unconnected server, got %d", len(resources))
	}
}

func TestListResources_Invoke_ServerNotFound(t *testing.T) {
	r := stubRegistry()
	mcpregistry.SetLastRegistry(r)

	tr := NewTool()
	result, err := tr.Invoke(context.Background(), tool.Call{
		Input: map[string]any{"server": "nonexistent"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Fatalf("expected error for unknown server")
	}
}

func TestListResources_Invoke_InvalidSchema(t *testing.T) {
	tr := NewTool()
	result, err := tr.Invoke(context.Background(), tool.Call{
		Input: map[string]any{"unknown_field": "value"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Fatalf("expected error for invalid input schema")
	}
}

func TestListResources_Invoke_EmptyInput(t *testing.T) {
	r := stubRegistry()
	mcpregistry.SetLastRegistry(r)

	tr := NewTool()
	result, err := tr.Invoke(context.Background(), tool.Call{Input: map[string]any{}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected tool error: %s", result.Error)
	}

	var resources []resourceEntry
	if err := json.Unmarshal([]byte(result.Output), &resources); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}
	if len(resources) != 0 {
		t.Fatalf("expected 0 resources, got %d", len(resources))
	}
}
