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

// NotificationHandler consumes a JSON-RPC notification emitted by the server.
type NotificationHandler func(JSONRPCNotification)

// RequestHandler consumes a JSON-RPC request emitted by the server and returns a response payload.
type RequestHandler func(JSONRPCRequest) (any, error)

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
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ClientCapabilities `json:"capabilities"`
	ClientInfo      Implementation     `json:"clientInfo"`
}

// ClientCapabilities declares what the client supports.
type ClientCapabilities struct {
	Roots       map[string]any `json:"roots,omitempty"`
	Sampling    map[string]any `json:"sampling,omitempty"`
	Elicitation map[string]any `json:"elicitation,omitempty"`
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
	Tools     *ToolsCapability     `json:"tools,omitempty"`
	Resources *ResourcesCapability `json:"resources,omitempty"`
	Prompts   *PromptsCapability   `json:"prompts,omitempty"`
	Logging   map[string]any       `json:"logging,omitempty"`
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
	Tools      []Tool `json:"tools"`
	NextCursor string `json:"nextCursor,omitempty"`
}

// Tool describes a single tool exposed by an MCP server.
type Tool struct {
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	InputSchema ToolInputSchema  `json:"inputSchema,omitempty"`
	Annotations *ToolAnnotations `json:"annotations,omitempty"`
}

// ListResourcesRequest requests the list of resources from a server.
type ListResourcesRequest struct {
	Cursor string `json:"cursor,omitempty"`
}

// ListResourcesResult is the response to a resources/list request.
type ListResourcesResult struct {
	Resources  []Resource `json:"resources"`
	NextCursor string     `json:"nextCursor,omitempty"`
}

// Resource describes a single resource exposed by an MCP server.
type Resource struct {
	URI         string `json:"uri"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

// ReadResourceRequest requests a single resource by URI.
type ReadResourceRequest struct {
	URI string `json:"uri"`
}

// ReadResourceResult is the response to a resources/read request.
type ReadResourceResult struct {
	Contents []map[string]any `json:"contents"`
}

// ListPromptsRequest requests the list of prompts from a server.
type ListPromptsRequest struct {
	Cursor string `json:"cursor,omitempty"`
}

// ListPromptsResult is the response to a prompts/list request.
type ListPromptsResult struct {
	Prompts    []Prompt `json:"prompts"`
	NextCursor string   `json:"nextCursor,omitempty"`
}

// Prompt describes a single prompt template exposed by an MCP server.
type Prompt struct {
	Name        string                    `json:"name"`
	Title       string                    `json:"title,omitempty"`
	Description string                    `json:"description,omitempty"`
	Arguments   map[string]PromptArgument `json:"arguments,omitempty"`
}

// PromptArgument describes one prompt argument.
type PromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

// GetPromptRequest requests one prompt template with optional arguments.
type GetPromptRequest struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

// GetPromptResult is the response to a prompts/get request.
type GetPromptResult struct {
	Description string           `json:"description,omitempty"`
	Messages    []map[string]any `json:"messages"`
}

// ToolInputSchema is the JSON Schema for a tool's input.
type ToolInputSchema struct {
	Type       string         `json:"type,omitempty"`
	Properties map[string]any `json:"properties,omitempty"`
	Required   []string       `json:"required,omitempty"`
}

// ToolAnnotations carries hints about tool behavior.
type ToolAnnotations struct {
	ReadOnlyHint    bool `json:"readOnlyHint,omitempty"`
	DestructiveHint bool `json:"destructiveHint,omitempty"`
	OpenWorldHint   bool `json:"openWorldHint,omitempty"`
}

// ElicitRequestMethod is the JSON-RPC method used by MCP servers to request user input.
const ElicitRequestMethod = "elicitation/create"

// ElicitationCompleteNotificationMethod is the JSON-RPC notification emitted when a URL-mode elicitation completes.
const ElicitationCompleteNotificationMethod = "notifications/elicitation/complete"

// ElicitRequestParams models the MCP elicitation request payload.
type ElicitRequestParams struct {
	// Mode indicates whether the elicitation is a "form" or "url" flow.
	Mode string `json:"mode"`
	// Message explains why the server is requesting user input.
	Message string `json:"message"`
	// RequestedSchema describes the expected response shape for form-mode elicitations.
	RequestedSchema map[string]any `json:"requestedSchema,omitempty"`
	// URL carries the external URL for URL-mode elicitations.
	URL string `json:"url,omitempty"`
	// ElicitationID identifies the URL-mode elicitation when one is provided.
	ElicitationID string `json:"elicitationId,omitempty"`
}

// ElicitResult is the MCP response returned to an elicitation request.
type ElicitResult struct {
	// Action is one of "accept", "decline", or "cancel".
	Action string `json:"action"`
	// Content carries submitted form values for accepted form-mode elicitations.
	Content map[string]any `json:"content,omitempty"`
}

// ElicitationCompleteNotification models the completion notification payload for URL-mode elicitations.
type ElicitationCompleteNotification struct {
	// ElicitationID identifies the completed elicitation.
	ElicitationID string `json:"elicitationId"`
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
	Content           []ContentItem  `json:"content"`
	IsError           bool           `json:"isError,omitempty"`
	Meta              map[string]any `json:"_meta,omitempty"`
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
