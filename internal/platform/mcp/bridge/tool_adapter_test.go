package bridge

import (
	"context"
	"errors"
	"os"
	"strings"
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

func (m *mockClientTransport) SetNotificationHandler(method string, handler client.NotificationHandler) {
}

// blockingMockTransport blocks until the context is cancelled, then returns
// the context error.  It is used to verify timeout behaviour.
type blockingMockTransport struct{}

func (b *blockingMockTransport) Send(ctx context.Context, req client.JSONRPCRequest) (*client.JSONRPCResponse, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func (b *blockingMockTransport) Close() error { return nil }

func (b *blockingMockTransport) SetNotificationHandler(method string, handler client.NotificationHandler) {
}

func TestProxyToolInvokeTimeout(t *testing.T) {
	// Set a very short timeout so the test does not hang.
	os.Setenv(envMcpToolTimeout, "50")
	defer os.Unsetenv(envMcpToolTimeout)

	c := client.NewClient(&blockingMockTransport{})
	pt := AdaptTool("srv", client.Tool{Name: "test"}, c)

	result, err := pt.Invoke(context.Background(), tool.Call{ID: "1", Input: map[string]any{}})
	if err != nil {
		t.Fatalf("invoke error: %v", err)
	}
	if result.Error == "" {
		t.Fatal("expected error result after timeout")
	}
	// The error should contain "deadline exceeded".
	if !strings.Contains(result.Error, "deadline exceeded") && !strings.Contains(result.Error, "context deadline exceeded") {
		t.Fatalf("expected deadline exceeded in error, got: %q", result.Error)
	}
}

func TestProxyToolInvokeProgress(t *testing.T) {
	mt := &mockClientTransport{
		responses: map[client.RequestID]client.JSONRPCResponse{
			"1": {
				JSONRPC: "2.0",
				ID:      "1",
				Result:  []byte(`{"content":[{"type":"text","text":"ok"}],"isError":false}`),
			},
		},
	}
	c := client.NewClient(mt)
	pt := AdaptTool("srv", client.Tool{Name: "test"}, c)

	var events []map[string]any
	progressFn := func(data any) {
		if m, ok := data.(map[string]any); ok {
			events = append(events, m)
		}
	}
	ctx := tool.WithProgress(context.Background(), progressFn)

	_, err := pt.Invoke(ctx, tool.Call{ID: "1", Input: map[string]any{}})
	if err != nil {
		t.Fatalf("invoke error: %v", err)
	}

	if len(events) != 2 {
		t.Fatalf("expected 2 progress events, got %d", len(events))
	}
	if events[0]["status"] != "started" {
		t.Fatalf("first event status = %q, want started", events[0]["status"])
	}
	if events[1]["status"] != "finished" {
		t.Fatalf("second event status = %q, want finished", events[1]["status"])
	}
}

func TestProxyToolInvokeAuthError(t *testing.T) {
	// Transport that returns an auth-like error.
	authTransport := &errorMockTransport{err: errors.New("server returned 401 Unauthorized")}
	c := client.NewClient(authTransport)
	pt := AdaptTool("srv", client.Tool{Name: "test"}, c)

	result, err := pt.Invoke(context.Background(), tool.Call{ID: "1", Input: map[string]any{}})
	if err != nil {
		t.Fatalf("invoke error: %v", err)
	}
	if result.Error == "" {
		t.Fatal("expected error result")
	}
	// The Meta should contain the classified error.
	metaErr, ok := result.Meta["error"]
	if !ok {
		t.Fatal("expected error in Meta")
	}
	if _, ok := metaErr.(*McpAuthError); !ok {
		t.Fatalf("expected *McpAuthError in Meta, got %T", metaErr)
	}
}

func TestProxyToolInvokeSessionExpiredError(t *testing.T) {
	authTransport := &errorMockTransport{err: errors.New("connection closed")}
	c := client.NewClient(authTransport)
	pt := AdaptTool("srv", client.Tool{Name: "test"}, c)

	result, err := pt.Invoke(context.Background(), tool.Call{ID: "1", Input: map[string]any{}})
	if err != nil {
		t.Fatalf("invoke error: %v", err)
	}
	metaErr, ok := result.Meta["error"]
	if !ok {
		t.Fatal("expected error in Meta")
	}
	if _, ok := metaErr.(*McpSessionExpiredError); !ok {
		t.Fatalf("expected *McpSessionExpiredError in Meta, got %T", metaErr)
	}
}

func TestProxyToolInvokeMcpToolCallError(t *testing.T) {
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
	metaErr, ok := result.Meta["error"]
	if !ok {
		t.Fatal("expected error in Meta")
	}
	if _, ok := metaErr.(*McpToolCallError); !ok {
		t.Fatalf("expected *McpToolCallError in Meta, got %T", metaErr)
	}
}

// errorMockTransport always returns a fixed error.
type errorMockTransport struct {
	err error
}

func (e *errorMockTransport) Send(ctx context.Context, req client.JSONRPCRequest) (*client.JSONRPCResponse, error) {
	return nil, e.err
}

func (e *errorMockTransport) Close() error { return nil }

func (e *errorMockTransport) SetNotificationHandler(method string, handler client.NotificationHandler) {
}
