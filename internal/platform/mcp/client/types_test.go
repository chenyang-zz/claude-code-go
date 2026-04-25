package client

import (
	"encoding/json"
	"testing"
)

func TestJSONRPCRequestMarshal(t *testing.T) {
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      "1",
		Method:  "initialize",
		Params:  json.RawMessage(`{"protocolVersion":"2024-11-05"}`),
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	want := `{"jsonrpc":"2.0","id":"1","method":"initialize","params":{"protocolVersion":"2024-11-05"}}`
	if string(data) != want {
		t.Fatalf("marshal = %s, want %s", data, want)
	}
}

func TestJSONRPCResponseUnmarshal(t *testing.T) {
	raw := `{"jsonrpc":"2.0","id":"1","result":{"protocolVersion":"2024-11-05"}}`
	var resp JSONRPCResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.JSONRPC != "2.0" {
		t.Fatalf("jsonrpc = %q, want 2.0", resp.JSONRPC)
	}
	if resp.ID != "1" {
		t.Fatalf("id = %q, want 1", resp.ID)
	}
	if string(resp.Result) != `{"protocolVersion":"2024-11-05"}` {
		t.Fatalf("result = %s", resp.Result)
	}
}

func TestJSONRPCResponseUnmarshalError(t *testing.T) {
	raw := `{"jsonrpc":"2.0","id":"2","error":{"code":-32600,"message":"Invalid Request"}}`
	var resp JSONRPCResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal error response: %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error object")
	}
	if resp.Error.Code != -32600 {
		t.Fatalf("error code = %d, want -32600", resp.Error.Code)
	}
	if resp.Error.Message != "Invalid Request" {
		t.Fatalf("error message = %q", resp.Error.Message)
	}
}

func TestInitializeResultUnmarshal(t *testing.T) {
	raw := `{"protocolVersion":"2024-11-05","capabilities":{"tools":{}},"serverInfo":{"name":"test","version":"1.0"}}`
	var result InitializeResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("unmarshal initialize result: %v", err)
	}
	if result.ProtocolVersion != "2024-11-05" {
		t.Fatalf("protocolVersion = %q", result.ProtocolVersion)
	}
	if result.ServerInfo.Name != "test" {
		t.Fatalf("serverInfo.name = %q", result.ServerInfo.Name)
	}
	if result.Capabilities.Tools == nil {
		t.Fatal("capabilities.tools should be non-nil")
	}
}

func TestListToolsResultUnmarshal(t *testing.T) {
	raw := `{"tools":[{"name":"read_file","description":"Read a file","inputSchema":{"type":"object","properties":{"path":{"type":"string"}},"required":["path"]}}]}`
	var result ListToolsResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("unmarshal listTools result: %v", err)
	}
	if len(result.Tools) != 1 {
		t.Fatalf("len(tools) = %d, want 1", len(result.Tools))
	}
	if result.Tools[0].Name != "read_file" {
		t.Fatalf("tool.name = %q", result.Tools[0].Name)
	}
}

func TestListResourcesResultUnmarshal(t *testing.T) {
	raw := `{"resources":[{"uri":"file:///tmp/a","name":"config","description":"Config file","mimeType":"text/plain"}]}`
	var result ListResourcesResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("unmarshal listResources result: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("len(resources) = %d, want 1", len(result.Resources))
	}
	if result.Resources[0].URI != "file:///tmp/a" {
		t.Fatalf("resource.uri = %q", result.Resources[0].URI)
	}
}

func TestListPromptsResultUnmarshal(t *testing.T) {
	raw := `{"prompts":[{"name":"summarize","description":"Summarize","arguments":{"path":{"name":"path","description":"Target file","required":true}}}]}`
	var result ListPromptsResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("unmarshal listPrompts result: %v", err)
	}
	if len(result.Prompts) != 1 {
		t.Fatalf("len(prompts) = %d, want 1", len(result.Prompts))
	}
	if result.Prompts[0].Name != "summarize" {
		t.Fatalf("prompt.name = %q", result.Prompts[0].Name)
	}
}

func TestReadResourceResultUnmarshal(t *testing.T) {
	raw := `{"contents":[{"uri":"file:///tmp/a","mimeType":"text/plain","text":"hello"}]}`
	var result ReadResourceResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("unmarshal readResource result: %v", err)
	}
	if len(result.Contents) != 1 {
		t.Fatalf("len(contents) = %d, want 1", len(result.Contents))
	}
}

func TestGetPromptResultUnmarshal(t *testing.T) {
	raw := `{"description":"Summarize","messages":[{"role":"user","content":[{"type":"text","text":"Hello"}]}]}`
	var result GetPromptResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("unmarshal getPrompt result: %v", err)
	}
	if len(result.Messages) != 1 {
		t.Fatalf("len(messages) = %d, want 1", len(result.Messages))
	}
}

func TestCallToolResultUnmarshal(t *testing.T) {
	raw := `{"content":[{"type":"text","text":"hello"}],"isError":false}`
	var result CallToolResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("unmarshal callTool result: %v", err)
	}
	if len(result.Content) != 1 {
		t.Fatalf("len(content) = %d, want 1", len(result.Content))
	}
	if result.Content[0].Type != "text" || result.Content[0].Text != "hello" {
		t.Fatalf("content[0] = %+v", result.Content[0])
	}
}

func TestServerConfigMarshal(t *testing.T) {
	cfg := ServerConfig{
		Command: "npx",
		Args:    []string{"-y", "@modelcontextprotocol/server-filesystem"},
		Env:     map[string]string{"FOO": "bar"},
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal server config: %v", err)
	}
	var back ServerConfig
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("round-trip unmarshal: %v", err)
	}
	if back.Command != cfg.Command {
		t.Fatalf("command = %q", back.Command)
	}
	if len(back.Args) != 2 || back.Args[0] != "-y" {
		t.Fatalf("args = %v", back.Args)
	}
}
