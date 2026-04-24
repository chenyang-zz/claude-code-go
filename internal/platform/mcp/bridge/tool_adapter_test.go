package bridge

import (
	"context"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/tool"
	"github.com/sheepzhao/claude-code-go/internal/platform/mcp/client"
)

func TestProxyToolName(t *testing.T) {
	pt := AdaptTool("files", client.Tool{Name: "read_file"}, nil)
	if pt.Name() != "files__read_file" {
		t.Fatalf("name = %q, want files__read_file", pt.Name())
	}
}

func TestProxyToolDescription(t *testing.T) {
	pt := AdaptTool("s", client.Tool{Name: "x", Description: "desc"}, nil)
	if pt.Description() != "desc" {
		t.Fatalf("description = %q", pt.Description())
	}
}

func TestProxyToolIsReadOnly(t *testing.T) {
	pt := AdaptTool("s", client.Tool{Name: "x", Annotations: &client.ToolAnnotations{ReadOnlyHint: true}}, nil)
	if !pt.IsReadOnly() {
		t.Fatal("expected read-only")
	}
}

func TestProxyToolInputSchema(t *testing.T) {
	pt := AdaptTool("s", client.Tool{
		Name: "x",
		InputSchema: client.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"path": map[string]any{"type": "string", "description": "file path"},
			},
			Required: []string{"path"},
		},
	}, nil)
	schema := pt.InputSchema()
	if len(schema.Properties) != 1 {
		t.Fatalf("len(properties) = %d, want 1", len(schema.Properties))
	}
	pathField, ok := schema.Properties["path"]
	if !ok {
		t.Fatal("missing path property")
	}
	if pathField.Type != tool.ValueKindString {
		t.Fatalf("path type = %q, want string", pathField.Type)
	}
	if !pathField.Required {
		t.Fatal("path should be required")
	}
}

func TestContentToString(t *testing.T) {
	items := []client.ContentItem{
		{Type: "text", Text: "hello"},
		{Type: "image", MimeType: "image/png"},
		{Type: "resource"},
		{Type: "unknown"},
	}
	got := contentToString(items)
	want := "hello\n[image: image/png]\n[resource]\n[unknown]"
	if got != want {
		t.Fatalf("contentToString = %q, want %q", got, want)
	}
}

func TestConvertInputSchema(t *testing.T) {
	schema := client.ToolInputSchema{
		Properties: map[string]any{
			"count": map[string]any{"type": "integer"},
			"flag":  map[string]any{"type": "boolean"},
		},
		Required: []string{"count"},
	}
	converted := convertInputSchema(schema)
	if len(converted.Properties) != 2 {
		t.Fatalf("len(properties) = %d, want 2", len(converted.Properties))
	}
	if converted.Properties["count"].Type != tool.ValueKindInteger {
		t.Fatalf("count type = %q", converted.Properties["count"].Type)
	}
	if !converted.Properties["count"].Required {
		t.Fatal("count should be required")
	}
	if converted.Properties["flag"].Required {
		t.Fatal("flag should not be required")
	}
}

func TestProxyToolInvokeErrorResult(t *testing.T) {
	mt := &mockClientTransport{
		responses: map[client.RequestID]client.JSONRPCResponse{
			"1": {
				JSONRPC: "2.0",
				ID:      "1",
				Result:  []byte(`{"content":[{"type":"text","text":"bad args"}],"isError":true}`),
			},
		},
	}
	c := client.NewClient(mt)
	pt := AdaptTool("srv", client.Tool{Name: "test"}, c)
	result, err := pt.Invoke(context.Background(), tool.Call{ID: "1", Input: map[string]any{}})
	if err != nil {
		t.Fatalf("invoke error: %v", err)
	}
	if result.Error == "" {
		t.Fatal("expected error result")
	}
}

// mockClientTransport implements client.Transport for testing.
type mockClientTransport struct {
	responses map[client.RequestID]client.JSONRPCResponse
}

func (m *mockClientTransport) Send(ctx context.Context, req client.JSONRPCRequest) (*client.JSONRPCResponse, error) {
	resp, ok := m.responses[req.ID]
	if !ok {
		return nil, nil
	}
	return &resp, nil
}

func (m *mockClientTransport) Close() error { return nil }
