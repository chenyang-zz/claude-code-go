package client

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// Transport defines the minimal interface for an MCP transport.
type Transport interface {
	// Send writes a JSON-RPC request and returns the matching response.
	Send(ctx context.Context, req JSONRPCRequest) (*JSONRPCResponse, error)
	// SetNotificationHandler registers a handler for an incoming notification method.
	SetNotificationHandler(method string, handler NotificationHandler)
	// SetRequestHandler registers a handler for an incoming request method.
	SetRequestHandler(method string, handler RequestHandler)
	// Close shuts down the transport and releases resources.
	Close() error
}

// StdioClientTransport runs an MCP server as a subprocess and communicates
// over stdin/stdout using line-delimited JSON-RPC 2.0 messages.
type StdioClientTransport struct {
	cmd                  *exec.Cmd
	stdin                io.WriteCloser
	stdout               io.ReadCloser
	stderr               io.ReadCloser
	pending              map[RequestID]chan JSONRPCResponse
	notificationHandlers map[string]NotificationHandler
	requestHandlers      map[string]RequestHandler
	mu                   sync.Mutex
	writeMu              sync.Mutex
	nextID               atomic.Int64
	reader               *bufio.Scanner
	closeCh              chan struct{}
	closed               atomic.Bool
	wg                   sync.WaitGroup
}

// NewStdioClientTransport starts the subprocess and returns a ready transport.
func NewStdioClientTransport(command string, args []string, env map[string]string) (*StdioClientTransport, error) {
	cmd := exec.Command(command, args...)

	// Merge optional extra env into the current process environment.
	if len(env) > 0 {
		base := make([]string, len(cmd.Environ()))
		copy(base, cmd.Environ())
		for k, v := range env {
			base = append(base, fmt.Sprintf("%s=%s", k, v))
		}
		cmd.Env = base
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("mcp stdio transport: stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("mcp stdio transport: stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("mcp stdio transport: stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("mcp stdio transport: start subprocess: %w", err)
	}

	t := &StdioClientTransport{
		cmd:                  cmd,
		stdin:                stdin,
		stdout:               stdout,
		stderr:               stderr,
		pending:              make(map[RequestID]chan JSONRPCResponse),
		notificationHandlers: make(map[string]NotificationHandler),
		requestHandlers:      make(map[string]RequestHandler),
		reader:               bufio.NewScanner(stdout),
		closeCh:              make(chan struct{}),
	}

	t.wg.Add(1)
	go t.readLoop()

	return t, nil
}

// Send writes a JSON-RPC request and waits for the matching response.
func (t *StdioClientTransport) Send(ctx context.Context, req JSONRPCRequest) (*JSONRPCResponse, error) {
	if t.closed.Load() {
		return nil, fmt.Errorf("mcp stdio transport: closed")
	}

	respCh := make(chan JSONRPCResponse, 1)

	t.mu.Lock()
	t.pending[req.ID] = respCh
	t.mu.Unlock()

	defer func() {
		t.mu.Lock()
		delete(t.pending, req.ID)
		t.mu.Unlock()
	}()

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("mcp stdio transport: marshal request: %w", err)
	}

	data = append(data, '\n')
	t.writeMu.Lock()
	if _, err := t.stdin.Write(data); err != nil {
		t.writeMu.Unlock()
		return nil, fmt.Errorf("mcp stdio transport: write request: %w", err)
	}
	t.writeMu.Unlock()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case resp := <-respCh:
		if resp.Error != nil {
			return nil, resp.Error
		}
		return &resp, nil
	}
}

// SetNotificationHandler registers a callback for an incoming notification method.
func (t *StdioClientTransport) SetNotificationHandler(method string, handler NotificationHandler) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if handler == nil {
		delete(t.notificationHandlers, method)
		return
	}
	t.notificationHandlers[method] = handler
}

// SetRequestHandler registers a callback for an incoming server request method.
func (t *StdioClientTransport) SetRequestHandler(method string, handler RequestHandler) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if handler == nil {
		delete(t.requestHandlers, method)
		return
	}
	t.requestHandlers[method] = handler
}

// Close terminates the subprocess and cleans up resources.
func (t *StdioClientTransport) Close() error {
	if t.closed.Swap(true) {
		return nil
	}
	close(t.closeCh)

	// Close stdin so the server sees EOF.
	_ = t.stdin.Close()

	// Give the subprocess a moment to exit gracefully.
	done := make(chan error, 1)
	go func() { done <- t.cmd.Wait() }()

	select {
	case <-done:
		// Exited cleanly.
	case <-time.After(5 * time.Second):
		_ = t.cmd.Process.Kill()
	}

	t.wg.Wait()
	return nil
}

// readLoop continuously reads line-delimited JSON-RPC responses from stdout
// and routes them to the pending request channels.
func (t *StdioClientTransport) readLoop() {
	defer t.wg.Done()

	for {
		select {
		case <-t.closeCh:
			return
		default:
		}

		if !t.reader.Scan() {
			if err := t.reader.Err(); err != nil {
				logger.WarnCF("mcp", "stdio transport read error", map[string]any{
					"error": err.Error(),
				})
			}
			return
		}

		line := t.reader.Bytes()
		if len(line) == 0 {
			continue
		}

		// Try incoming request first so server-initiated RPC calls do not get
		// mistaken for responses.
		var req JSONRPCRequest
		if err := json.Unmarshal(line, &req); err == nil && req.Method != "" && req.ID != "" {
			t.handleRequest(req)
			continue
		}

		var resp JSONRPCResponse
		if err := json.Unmarshal(line, &resp); err == nil && resp.ID != "" {
			t.mu.Lock()
			ch, ok := t.pending[resp.ID]
			t.mu.Unlock()

			if ok {
				ch <- resp
			} else {
				logger.DebugCF("mcp", "stdio transport orphan response", map[string]any{
					"id": resp.ID,
				})
			}
			continue
		}

		var notif JSONRPCNotification
		if err := json.Unmarshal(line, &notif); err == nil && notif.Method != "" {
			t.mu.Lock()
			handler := t.notificationHandlers[notif.Method]
			t.mu.Unlock()
			if handler != nil {
				go handler(notif)
			} else {
				logger.DebugCF("mcp", "stdio transport unhandled notification", map[string]any{
					"method": notif.Method,
				})
			}
			continue
		}

		logger.DebugCF("mcp", "stdio transport unparseable line", map[string]any{
			"line": string(line),
		})
	}
}

// handleRequest dispatches one incoming JSON-RPC request to the registered handler.
func (t *StdioClientTransport) handleRequest(req JSONRPCRequest) {
	t.mu.Lock()
	handler := t.requestHandlers[req.Method]
	t.mu.Unlock()

	if handler == nil {
		logger.DebugCF("mcp", "stdio transport unhandled request", map[string]any{
			"method": req.Method,
			"id":     req.ID,
		})
		t.writeResponse(JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &JSONRPCError{
				Code:    -32601,
				Message: fmt.Sprintf("mcp stdio transport: unhandled request %q", req.Method),
			},
		})
		return
	}

	go func() {
		result, err := handler(req)
		if err != nil {
			t.writeResponse(JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error: &JSONRPCError{
					Code:    -32000,
					Message: err.Error(),
				},
			})
			return
		}
		t.writeResponse(JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  mustMarshalRaw(result),
		})
	}()
}

// writeResponse serializes one JSON-RPC response back to the subprocess stdin.
func (t *StdioClientTransport) writeResponse(resp JSONRPCResponse) {
	data, err := json.Marshal(resp)
	if err != nil {
		logger.WarnCF("mcp", "stdio transport marshal response failed", map[string]any{
			"id":    resp.ID,
			"error": err.Error(),
		})
		return
	}

	data = append(data, '\n')
	t.writeMu.Lock()
	defer t.writeMu.Unlock()
	if _, err := t.stdin.Write(data); err != nil {
		logger.WarnCF("mcp", "stdio transport write response failed", map[string]any{
			"id":    resp.ID,
			"error": err.Error(),
		})
	}
}

// mustMarshalRaw converts a handler result into JSON raw bytes when possible.
func mustMarshalRaw(v any) json.RawMessage {
	if v == nil {
		return nil
	}
	data, err := json.Marshal(v)
	if err != nil {
		logger.WarnCF("mcp", "stdio transport marshal request handler result failed", map[string]any{
			"error": err.Error(),
		})
		return nil
	}
	return json.RawMessage(data)
}

// NextID returns a monotonically increasing request ID.
func (t *StdioClientTransport) NextID() RequestID {
	return RequestID(fmt.Sprintf("%d", t.nextID.Add(1)))
}
