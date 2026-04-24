package client

import "encoding/json"

// JSON-RPC 2.0 base types for the MCP wire protocol.

// RequestID identifies a JSON-RPC request/response pair.
type RequestID string

// JSONRPCRequest is a JSON-RPC 2.0 request object.
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      RequestID       `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// JSONRPCResponse is a JSON-RPC 2.0 response object.
type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      RequestID       `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

// JSONRPCNotification is a JSON-RPC 2.0 notification (no ID).
type JSONRPCNotification struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// JSONRPCError is the error object inside a JSON-RPC response.
type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func (e *JSONRPCError) Error() string {
	return e.Message
}

// MCP Initialize types.

// InitializeRequest is sent by the client to begin an MCP session.
type InitializeRequest struct {
	ProtocolVersion string              `json:"protocolVersion"`
	Capabilities    ClientCapabilities  `json:"capabilities"`
	ClientInfo      Implementation      `json:"clientInfo"`
}

// ClientCapabilities declares what the client supports.
type ClientCapabilities struct {
	Roots        map[string]any `json:"roots,omitempty"`
	Sampling     map[string]any `json:"sampling,omitempty"`
	Elicitation  map[string]any `json:"elicitation,omitempty"`
}

// Implementation describes a client or server implementation.
type Implementation struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// InitializeResult is returned by the server after a successful initialize.
type InitializeResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ServerCapabilities `json:"capabilities"`
	ServerInfo      Implementation     `json:"serverInfo"`
	Instructions    string             `json:"instructions,omitempty"`
}

// ServerCapabilities declares what the server supports.
type ServerCapabilities struct {
	Tools      *ToolsCapability      `json:"tools,omitempty"`
	Resources  *ResourcesCapability  `json:"resources,omitempty"`
	Prompts    *PromptsCapability    `json:"prompts,omitempty"`
	Logging    map[string]any        `json:"logging,omitempty"`
}

// ToolsCapability indicates the server exposes callable tools.
type ToolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// ResourcesCapability indicates the server exposes readable resources.
type ResourcesCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// PromptsCapability indicates the server exposes prompt templates.
type PromptsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// MCP tools/list types.

// ListToolsRequest requests the list of tools from a server.
type ListToolsRequest struct {
	Cursor string `json:"cursor,omitempty"`
}

// ListToolsResult is the response to a tools/list request.
type ListToolsResult struct {
	Tools    []Tool   `json:"tools"`
	NextCursor string `json:"nextCursor,omitempty"`
}

// Tool describes a single tool exposed by an MCP server.
type Tool struct {
	Name        string        `json:"name"`
	Description string        `json:"description,omitempty"`
	InputSchema ToolInputSchema `json:"inputSchema,omitempty"`
	Annotations *ToolAnnotations `json:"annotations,omitempty"`
}

// ToolInputSchema is the JSON Schema for a tool's input.
type ToolInputSchema struct {
	Type       string              `json:"type,omitempty"`
	Properties map[string]any      `json:"properties,omitempty"`
	Required   []string            `json:"required,omitempty"`
}

// ToolAnnotations carries hints about tool behavior.
type ToolAnnotations struct {
	ReadOnlyHint     bool `json:"readOnlyHint,omitempty"`
	DestructiveHint  bool `json:"destructiveHint,omitempty"`
	OpenWorldHint    bool `json:"openWorldHint,omitempty"`
}

// MCP tools/call types.

// CallToolRequest invokes a tool on the server.
type CallToolRequest struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
	Meta      map[string]any `json:"_meta,omitempty"`
}

// CallToolResult is the response from a tool invocation.
type CallToolResult struct {
	Content          []ContentItem `json:"content"`
	IsError          bool          `json:"isError,omitempty"`
	Meta             map[string]any `json:"_meta,omitempty"`
	StructuredContent map[string]any `json:"structuredContent,omitempty"`
}

// ContentItem is one element inside a CallToolResult.
type ContentItem struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	Data     string `json:"data,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
}

// ServerConfig describes a single MCP server entry from settings.
type ServerConfig struct {
	Type    string            `json:"type,omitempty"`
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}
