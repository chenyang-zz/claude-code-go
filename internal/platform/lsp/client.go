package lsp

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// NotificationHandler is a callback that receives the raw JSON params of an
// incoming LSP notification from the server.
type NotificationHandler func(rawParams json.RawMessage)

// Client manages communication with a single LSP server via stdio transport.
// It handles the initialize/shutdown handshake and JSON-RPC message exchange.
type Client struct {
	transport    *StdioTransport
	capabilities *ServerCapabilities
	isInitialized bool

	mu             sync.Mutex
	pending        map[int64]chan *Response
	notifHandlers  map[string][]NotificationHandler // method → handlers
}

// NewClient creates a new LSP client with an underlying stdio transport.
func NewClient() *Client {
	return &Client{
		transport:     &StdioTransport{},
		pending:       make(map[int64]chan *Response),
		notifHandlers: make(map[string][]NotificationHandler),
	}
}

// Start launches the LSP server process and begins reading responses.
func (c *Client) Start(command string, args ...string) error {
	if err := c.transport.Start(command, args...); err != nil {
		return err
	}
	go c.readLoop()
	return nil
}

// Initialize performs the LSP initialize handshake and returns the server
// capabilities. It sends the initialize request followed by the
// initialized notification.
func (c *Client) Initialize(rootURI string) (*InitializeResult, error) {
	params := map[string]any{
		"processId": 0,
		"capabilities": map[string]any{
			"textDocument": map[string]any{
				"synchronization": map[string]any{
					"dynamicRegistration": false,
					"willSave":            false,
					"willSaveWaitUntil":   false,
					"didSave":             true,
				},
				"hover": map[string]any{
					"dynamicRegistration": false,
					"contentFormat":       []string{"markdown", "plaintext"},
				},
				"definition": map[string]any{
					"dynamicRegistration": false,
					"linkSupport":         true,
				},
				"references": map[string]any{
					"dynamicRegistration": false,
				},
				"documentSymbol": map[string]any{
					"dynamicRegistration":            false,
					"hierarchicalDocumentSymbolSupport": true,
				},
				"callHierarchy": map[string]any{
					"dynamicRegistration": false,
				},
			},
			"workspace": map[string]any{
				"configuration":  false,
				"workspaceFolders": false,
			},
			"general": map[string]any{
				"positionEncodings": []string{"utf-16"},
			},
		},
	}
	if rootURI != "" {
		params["rootUri"] = rootURI
	}

	rawResult, err := c.SendRequest("initialize", params)
	if err != nil {
		return nil, fmt.Errorf("lsp initialize: %w", err)
	}

	var capsMap map[string]any
	if err := json.Unmarshal(rawResult, &capsMap); err != nil {
		return nil, fmt.Errorf("lsp initialize parse result: %w", err)
	}

	var result InitializeResult
	caps, ok := capsMap["capabilities"]
	if ok {
		if cm, ok := caps.(map[string]any); ok {
			c.capabilities = parseServerCapabilities(cm)
			result.Capabilities = *c.capabilities
		}
	}

	// Send the initialized notification.
	if err := c.SendNotification("initialized", nil); err != nil {
		logger.DebugCF("lsp.client", "initialized notification failed", map[string]any{
			"error": err.Error(),
		})
	}

	c.isInitialized = true
	logger.DebugCF("lsp.client", "initialized", map[string]any{
		"rootURI": rootURI,
	})

	return &result, nil
}

// Shutdown performs the graceful shutdown handshake and closes the transport.
func (c *Client) Shutdown() error {
	c.mu.Lock()
	if !c.isInitialized {
		c.mu.Unlock()
		c.transport.Close()
		return nil
	}
	c.isInitialized = false
	c.capabilities = nil
	c.mu.Unlock()

	// Try graceful shutdown.
	if _, err := c.SendRequest("shutdown", nil); err != nil {
		logger.DebugCF("lsp.client", "shutdown request failed", map[string]any{
			"error": err.Error(),
		})
	}
	_ = c.SendNotification("exit", nil)

	return c.transport.Close()
}

// SendRequest sends a JSON-RPC request and waits for the matching response.
// Params can be a struct, map, or nil. Result is returned as raw JSON.
func (c *Client) SendRequest(method string, params any) (json.RawMessage, error) {
	// Marshal params to JSON first, then unmarshal to map for the request envelope.
	rawParams, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("lsp send request marshal params: %w", err)
	}
	var paramsMap map[string]any
	if params != nil {
		if err := json.Unmarshal(rawParams, &paramsMap); err != nil {
			return nil, fmt.Errorf("lsp send request unmarshal params: %w", err)
		}
	}

	req := NewRequest(method, paramsMap)
	respCh := make(chan *Response, 1)

	c.mu.Lock()
	c.pending[req.ID] = respCh
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		delete(c.pending, req.ID)
		c.mu.Unlock()
	}()

	if err := WriteMessage(c.transport.Writer(), req); err != nil {
		return nil, fmt.Errorf("lsp send request: %w", err)
	}

	resp := <-respCh
	if resp == nil {
		return nil, fmt.Errorf("lsp: request %q (id=%d) cancelled", method, req.ID)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("lsp: %s (code=%d)", resp.Error.Message, resp.Error.Code)
	}

	return resp.Result, nil
}

// SendNotification sends a JSON-RPC notification. Notifications are
// fire-and-forget — no response is expected.
func (c *Client) SendNotification(method string, params any) error {
	var paramsMap map[string]any
	if params != nil {
		raw, err := json.Marshal(params)
		if err != nil {
			return fmt.Errorf("lsp notification marshal params: %w", err)
		}
		if err := json.Unmarshal(raw, &paramsMap); err != nil {
			return fmt.Errorf("lsp notification unmarshal params: %w", err)
		}
	}
	notif := NewNotification(method, paramsMap)
	return WriteMessage(c.transport.Writer(), notif)
}

// OnNotification registers a handler for incoming notifications from the server
// with the given method name (e.g., "textDocument/publishDiagnostics").
// Multiple handlers can be registered for the same method; they are called in
// registration order.
func (c *Client) OnNotification(method string, handler NotificationHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.notifHandlers[method] = append(c.notifHandlers[method], handler)
}

// Capabilities returns the server capabilities discovered during initialize.
func (c *Client) Capabilities() *ServerCapabilities {
	return c.capabilities
}

// incomingMessage is a generic JSON-RPC message used to detect whether
// an incoming server message is a response (has id + result/error) or
// a notification (has method, no id).
type incomingMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int64          `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// readLoop continuously reads JSON-RPC messages from the server's stdout
// and dispatches them to pending request response channels or notification
// handlers depending on whether the message is a response or a notification.
func (c *Client) readLoop() {
	reader := c.transport.Reader()
	if reader == nil {
		return
	}

	for {
		var msg incomingMessage
		if err := ReadMessage(reader, &msg); err != nil {
			logger.DebugCF("lsp.client", "read loop ended", map[string]any{
				"error": err.Error(),
			})
			return
		}

		if msg.Method != "" && msg.ID == nil {
			// Server-to-client notification (e.g., textDocument/publishDiagnostics).
			c.mu.Lock()
			handlers := c.notifHandlers[msg.Method]
			c.mu.Unlock()

			for _, handler := range handlers {
				handler(msg.Params)
			}
		} else if msg.ID != nil {
			// Server-to-client response.
			resp := &Response{
				JSONRPC: msg.JSONRPC,
				ID:      *msg.ID,
				Result:  msg.Result,
				Error:   msg.Error,
			}

			c.mu.Lock()
			ch, ok := c.pending[resp.ID]
			c.mu.Unlock()

			if ok {
				ch <- resp
			}
		}
	}
}

// parseServerCapabilities extracts ServerCapabilities from the raw response map.
func parseServerCapabilities(m map[string]any) *ServerCapabilities {
	caps := &ServerCapabilities{}
	if v, ok := m["definitionProvider"].(bool); ok {
		caps.DefinitionProvider = v
	}
	if v, ok := m["referencesProvider"].(bool); ok {
		caps.ReferencesProvider = v
	}
	if v, ok := m["hoverProvider"].(bool); ok {
		caps.HoverProvider = v
	}
	if v, ok := m["documentSymbolProvider"].(bool); ok {
		caps.DocumentSymbolProvider = v
	}
	if v, ok := m["workspaceSymbolProvider"].(bool); ok {
		caps.WorkspaceSymbolProvider = v
	}
	if v, ok := m["implementationProvider"].(bool); ok {
		caps.ImplementationProvider = v
	}
	if v, ok := m["callHierarchyProvider"].(bool); ok {
		caps.CallHierarchyProvider = v
	}
	return caps
}
