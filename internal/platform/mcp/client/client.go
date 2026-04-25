package client

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

// Client manages an MCP session over a Transport.
type Client struct {
	transport Transport
	mu        sync.RWMutex
	closed    bool
}

// NewClient creates a client bound to the given transport.
func NewClient(t Transport) *Client {
	return &Client{transport: t}
}

// SetNotificationHandler registers a callback for a server notification method.
func (c *Client) SetNotificationHandler(method string, handler NotificationHandler) error {
	if c.transport == nil {
		return fmt.Errorf("mcp client: nil transport")
	}
	c.transport.SetNotificationHandler(method, handler)
	return nil
}

// SetRequestHandler registers a callback for an incoming server request method.
func (c *Client) SetRequestHandler(method string, handler RequestHandler) error {
	if c.transport == nil {
		return fmt.Errorf("mcp client: nil transport")
	}
	c.transport.SetRequestHandler(method, handler)
	return nil
}

// Initialize performs the MCP initialization handshake.
func (c *Client) Initialize(ctx context.Context, req InitializeRequest) (*InitializeResult, error) {
	params, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("mcp client: marshal initialize request: %w", err)
	}

	resp, err := c.request(ctx, "initialize", params)
	if err != nil {
		return nil, fmt.Errorf("mcp client: initialize: %w", err)
	}

	var result InitializeResult
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("mcp client: unmarshal initialize result: %w", err)
	}
	return &result, nil
}

// ListTools fetches the list of tools exposed by the server.
func (c *Client) ListTools(ctx context.Context) (*ListToolsResult, error) {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return nil, fmt.Errorf("mcp client: closed")
	}
	c.mu.RUnlock()

	req := ListToolsRequest{}
	params, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("mcp client: marshal listTools request: %w", err)
	}

	resp, err := c.request(ctx, "tools/list", params)
	if err != nil {
		return nil, fmt.Errorf("mcp client: listTools: %w", err)
	}

	var result ListToolsResult
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("mcp client: unmarshal listTools result: %w", err)
	}
	return &result, nil
}

// ListResources fetches the list of resources exposed by the server.
func (c *Client) ListResources(ctx context.Context) (*ListResourcesResult, error) {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return nil, fmt.Errorf("mcp client: closed")
	}
	c.mu.RUnlock()

	req := ListResourcesRequest{}
	params, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("mcp client: marshal listResources request: %w", err)
	}

	resp, err := c.request(ctx, "resources/list", params)
	if err != nil {
		return nil, fmt.Errorf("mcp client: listResources: %w", err)
	}

	var result ListResourcesResult
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("mcp client: unmarshal listResources result: %w", err)
	}
	return &result, nil
}

// ReadResource reads a single resource by URI.
func (c *Client) ReadResource(ctx context.Context, req ReadResourceRequest) (*ReadResourceResult, error) {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return nil, fmt.Errorf("mcp client: closed")
	}
	c.mu.RUnlock()

	params, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("mcp client: marshal readResource request: %w", err)
	}

	resp, err := c.request(ctx, "resources/read", params)
	if err != nil {
		return nil, fmt.Errorf("mcp client: readResource: %w", err)
	}

	var result ReadResourceResult
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("mcp client: unmarshal readResource result: %w", err)
	}
	return &result, nil
}

// ListPrompts fetches the list of prompts exposed by the server.
func (c *Client) ListPrompts(ctx context.Context) (*ListPromptsResult, error) {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return nil, fmt.Errorf("mcp client: closed")
	}
	c.mu.RUnlock()

	req := ListPromptsRequest{}
	params, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("mcp client: marshal listPrompts request: %w", err)
	}

	resp, err := c.request(ctx, "prompts/list", params)
	if err != nil {
		return nil, fmt.Errorf("mcp client: listPrompts: %w", err)
	}

	var result ListPromptsResult
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("mcp client: unmarshal listPrompts result: %w", err)
	}
	return &result, nil
}

// GetPrompt fetches a single prompt template and its rendered messages.
func (c *Client) GetPrompt(ctx context.Context, req GetPromptRequest) (*GetPromptResult, error) {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return nil, fmt.Errorf("mcp client: closed")
	}
	c.mu.RUnlock()

	params, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("mcp client: marshal getPrompt request: %w", err)
	}

	resp, err := c.request(ctx, "prompts/get", params)
	if err != nil {
		return nil, fmt.Errorf("mcp client: getPrompt: %w", err)
	}

	var result GetPromptResult
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("mcp client: unmarshal getPrompt result: %w", err)
	}
	return &result, nil
}

// CallTool invokes a named tool on the server.
func (c *Client) CallTool(ctx context.Context, req CallToolRequest) (*CallToolResult, error) {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return nil, fmt.Errorf("mcp client: closed")
	}
	c.mu.RUnlock()

	params, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("mcp client: marshal callTool request: %w", err)
	}

	resp, err := c.request(ctx, "tools/call", params)
	if err != nil {
		return nil, fmt.Errorf("mcp client: callTool: %w", err)
	}

	var result CallToolResult
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("mcp client: unmarshal callTool result: %w", err)
	}
	return &result, nil
}

// Close shuts down the transport.
func (c *Client) Close() error {
	c.mu.Lock()
	c.closed = true
	c.mu.Unlock()
	if c.transport != nil {
		return c.transport.Close()
	}
	return nil
}

// request sends a JSON-RPC request with an auto-generated ID and returns the raw result.
func (c *Client) request(ctx context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return nil, fmt.Errorf("mcp client: closed")
	}
	c.mu.RUnlock()

	// Obtain the next request ID from the transport when available.
	var id RequestID
	if t, ok := c.transport.(interface{ NextID() RequestID }); ok {
		id = t.NextID()
	} else {
		// Fallback: use a simple counter. In practice StdioClientTransport always implements NextID.
		id = RequestID("1")
	}

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	resp, err := c.transport.Send(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, fmt.Errorf("mcp client: nil response from transport")
	}
	if resp.Error != nil {
		return nil, resp.Error
	}
	return resp.Result, nil
}
