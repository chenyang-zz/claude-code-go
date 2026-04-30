package read_resource

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/tool"
	mcpclient "github.com/sheepzhao/claude-code-go/internal/platform/mcp/client"
	mcpregistry "github.com/sheepzhao/claude-code-go/internal/platform/mcp/registry"
)

// stubTransport implements client.Transport for testing.
type stubTransport struct{}

func (s *stubTransport) Send(ctx context.Context, req mcpclient.JSONRPCRequest) (*mcpclient.JSONRPCResponse, error) {
	if req.Method == "resources/read" {
		resp := map[string]any{
			"contents": []map[string]any{
				{
					"uri":      "test://resource",
					"mimeType": "text/plain",
					"text":     "hello from resource",
				},
			},
		}
		b, _ := json.Marshal(resp)
		return &mcpclient.JSONRPCResponse{Result: b}, nil
	}
	return &mcpclient.JSONRPCResponse{}, nil
}
func (s *stubTransport) Close() error { return nil }
func (s *stubTransport) SetNotificationHandler(method string, handler mcpclient.NotificationHandler) {
}
func (s *stubTransport) SetRequestHandler(method string, handler mcpclient.RequestHandler) {}

func TestReadResource_Name(t *testing.T) {
	tr := NewTool()
	if tr.Name() != Name {
		t.Fatalf("expected Name %q, got %q", Name, tr.Name())
	}
}

func TestReadResource_Description(t *testing.T) {
	tr := NewTool()
	if tr.Description() == "" {
		t.Fatalf("expected non-empty Description")
	}
}

func TestReadResource_InputSchema(t *testing.T) {
	tr := NewTool()
	schema := tr.InputSchema()
	if _, ok := schema.Properties["server"]; !ok {
		t.Fatalf("expected 'server' property in input schema")
	}
	if _, ok := schema.Properties["uri"]; !ok {
		t.Fatalf("expected 'uri' property in input schema")
	}
	if !schema.Properties["server"].Required {
		t.Fatalf("expected 'server' to be required")
	}
	if !schema.Properties["uri"].Required {
		t.Fatalf("expected 'uri' to be required")
	}
}

func TestReadResource_IsReadOnly(t *testing.T) {
	tr := NewTool()
	if !tr.IsReadOnly() {
		t.Fatalf("expected IsReadOnly to return true")
	}
}

func TestReadResource_IsConcurrencySafe(t *testing.T) {
	tr := NewTool()
	if !tr.IsConcurrencySafe() {
		t.Fatalf("expected IsConcurrencySafe to return true")
	}
}

func TestReadResource_Invoke_RegistryNil(t *testing.T) {
	mcpregistry.SetLastRegistry(nil)

	tr := NewTool()
	result, err := tr.Invoke(context.Background(), tool.Call{
		Input: map[string]any{"server": "s", "uri": "u"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Fatalf("expected error for nil registry")
	}
}

func TestReadResource_Invoke_ServerNotFound(t *testing.T) {
	r := mcpregistry.NewServerRegistry()
	mcpregistry.SetLastRegistry(r)

	tr := NewTool()
	result, err := tr.Invoke(context.Background(), tool.Call{
		Input: map[string]any{"server": "nonexistent", "uri": "test://uri"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Fatalf("expected error for unknown server")
	}
}

func TestReadResource_Invoke_ServerNotConnected(t *testing.T) {
	r := mcpregistry.NewServerRegistry()
	r.LoadConfigs(map[string]mcpclient.ServerConfig{
		"offline": {Command: "echo", Args: []string{}},
	})
	mcpregistry.SetLastRegistry(r)

	tr := NewTool()
	result, err := tr.Invoke(context.Background(), tool.Call{
		Input: map[string]any{"server": "offline", "uri": "test://uri"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Fatalf("expected error for disconnected server")
	}
}

func TestReadResource_Invoke_MissingServer(t *testing.T) {
	tr := NewTool()
	result, err := tr.Invoke(context.Background(), tool.Call{
		Input: map[string]any{"uri": "test://uri"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Fatalf("expected error for missing server field")
	}
}

func TestReadResource_Invoke_MissingURI(t *testing.T) {
	tr := NewTool()
	result, err := tr.Invoke(context.Background(), tool.Call{
		Input: map[string]any{"server": "test-server"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Fatalf("expected error for missing uri field")
	}
}

func TestReadResource_Invoke_InvalidInput(t *testing.T) {
	tr := NewTool()
	result, err := tr.Invoke(context.Background(), tool.Call{
		Input: map[string]any{"unknown": "value"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Fatalf("expected error for invalid input")
	}
}
