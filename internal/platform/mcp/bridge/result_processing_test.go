package bridge

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	"github.com/sheepzhao/claude-code-go/internal/platform/mcp/client"
)

func TestNormalizeMcpResultStructuredContent(t *testing.T) {
	result, err := normalizeMcpResult(&client.CallToolResult{
		StructuredContent: map[string]any{
			"title": "hello",
			"items": []any{"one", "two"},
		},
	})
	if err != nil {
		t.Fatalf("normalizeMcpResult() error = %v", err)
	}
	if result.Kind != resultKindStructuredContent {
		t.Fatalf("Kind = %q, want %q", result.Kind, resultKindStructuredContent)
	}
	if !result.Persistable {
		t.Fatal("expected structured content to be persistable")
	}
	if result.FormatDescription != "JSON" {
		t.Fatalf("FormatDescription = %q, want JSON", result.FormatDescription)
	}
	if result.TokenEstimate <= 0 {
		t.Fatalf("TokenEstimate = %d, want positive", result.TokenEstimate)
	}
}

func TestNormalizeMcpResultContentArray(t *testing.T) {
	result, err := normalizeMcpResult(&client.CallToolResult{
		Content: []client.ContentItem{
			{Type: "text", Text: "hello"},
			{Type: "text", Text: "world"},
		},
	})
	if err != nil {
		t.Fatalf("normalizeMcpResult() error = %v", err)
	}
	if result.Kind != resultKindContentArray {
		t.Fatalf("Kind = %q, want %q", result.Kind, resultKindContentArray)
	}
	if !result.Persistable {
		t.Fatal("expected text-only content array to be persistable")
	}
	if got, want := result.InlineText, "hello\nworld"; got != want {
		t.Fatalf("InlineText = %q, want %q", got, want)
	}
	if len(result.PersistBytes) == 0 {
		t.Fatal("PersistBytes should be populated for persistable arrays")
	}
}

func TestNormalizeMcpResultRejectsEmptyResult(t *testing.T) {
	_, err := normalizeMcpResult(&client.CallToolResult{})
	if err == nil {
		t.Fatal("expected error for empty MCP result")
	}
}

func TestProxyToolInvokePersistsLargeStructuredContent(t *testing.T) {
	t.Setenv(envMaxMcpOutputTokens, "1")
	t.Setenv(envEnableLargeMcpOutputs, "1")

	tempDir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", tempDir)

	mt := &mockClientTransport{
		responses: map[client.RequestID]client.JSONRPCResponse{
			"1": {
				JSONRPC: "2.0",
				ID:      "1",
				Result:  json.RawMessage(`{"structuredContent":{"title":"very-large","items":["` + strings.Repeat("abcdef", 80) + `"]},"content":[],"isError":false}`),
			},
		},
	}

	c := client.NewClient(mt)
	pt := AdaptTool("srv", client.Tool{Name: "test"}, c)

	call := coretool.Call{
		ID:    "call-1",
		Name:  "test",
		Input: map[string]any{},
		Context: coretool.UseContext{
			WorkingDir: "/tmp/project",
		},
	}

	result, err := pt.Invoke(context.Background(), call)
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result.Error != "" {
		t.Fatalf("Invoke() error result = %q, want empty", result.Error)
	}
	if !strings.Contains(result.Output, "saved to") {
		t.Fatalf("Invoke() output = %q, want persisted-output guidance", result.Output)
	}

	expectedPath := buildMcpOutputPath(
		call.Context.WorkingDir,
		"srv",
		"test",
		call.ID,
		normalizedMcpResult{
			Kind:              resultKindStructuredContent,
			InlineText:        "",
			PersistBytes:      nil,
			Persistable:       true,
			FormatDescription: "JSON",
		},
	)
	if _, err := os.Stat(expectedPath); err != nil {
		t.Fatalf("expected persisted file at %s: %v", expectedPath, err)
	}

	data, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("persisted file should contain JSON, got error: %v", err)
	}
	if decoded["title"] != "very-large" {
		t.Fatalf("persisted title = %v, want very-large", decoded["title"])
	}
}

func TestProxyToolInvokeTruncatesWhenLargeOutputDisabled(t *testing.T) {
	t.Setenv(envMaxMcpOutputTokens, "1")
	t.Setenv(envEnableLargeMcpOutputs, "0")

	mt := &mockClientTransport{
		responses: map[client.RequestID]client.JSONRPCResponse{
			"1": {
				JSONRPC: "2.0",
				ID:      "1",
				Result:  json.RawMessage(`{"content":[{"type":"text","text":"` + strings.Repeat("abc", 120) + `"}],"isError":false}`),
			},
		},
	}

	c := client.NewClient(mt)
	pt := AdaptTool("srv", client.Tool{Name: "test"}, c)

	result, err := pt.Invoke(context.Background(), coretool.Call{
		ID:    "call-2",
		Name:  "test",
		Input: map[string]any{},
		Context: coretool.UseContext{
			WorkingDir: "/tmp/project",
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result.Error != "" {
		t.Fatalf("Invoke() error result = %q, want empty", result.Error)
	}
	if !strings.Contains(result.Output, "[OUTPUT TRUNCATED") {
		t.Fatalf("Invoke() output = %q, want truncation warning", result.Output)
	}
}

func TestProxyToolInvokeRejectsUnexpectedResponseFormat(t *testing.T) {
	mt := &mockClientTransport{
		responses: map[client.RequestID]client.JSONRPCResponse{
			"1": {
				JSONRPC: "2.0",
				ID:      "1",
				Result:  json.RawMessage(`{"content":[],"isError":false}`),
			},
		},
	}

	c := client.NewClient(mt)
	pt := AdaptTool("srv", client.Tool{Name: "test"}, c)

	result, err := pt.Invoke(context.Background(), coretool.Call{
		ID:    "call-3",
		Name:  "test",
		Input: map[string]any{},
		Context: coretool.UseContext{
			WorkingDir: "/tmp/project",
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result.Error == "" {
		t.Fatal("expected validation error for empty MCP result")
	}
	if !strings.Contains(result.Error, "unexpected response format") {
		t.Fatalf("Invoke() error = %q, want unexpected response format", result.Error)
	}
}
