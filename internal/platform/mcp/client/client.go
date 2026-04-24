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
