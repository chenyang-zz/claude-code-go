package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/platform/remote"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

type remoteJSONRPCStream interface {
	Recv(ctx context.Context) (remote.Event, error)
	Close() error
}

type remoteJSONRPCTransport struct {
	stream  remoteJSONRPCStream
	sendRaw func(ctx context.Context, data []byte) error

	pending              map[RequestID]chan JSONRPCResponse
	notificationHandlers map[string]NotificationHandler
	requestHandlers      map[string]RequestHandler

	mu      sync.Mutex
	writeMu sync.Mutex

	readCtx    context.Context
	readCancel context.CancelFunc
	closeOnce  sync.Once
	closed     atomic.Bool
	wg         sync.WaitGroup
}

// newRemoteJSONRPCTransport wraps a bidirectional remote stream with the MCP transport interface.
func newRemoteJSONRPCTransport(
	stream remoteJSONRPCStream,
	sendRaw func(ctx context.Context, data []byte) error,
) *remoteJSONRPCTransport {
	readCtx, readCancel := context.WithCancel(context.Background())
	t := &remoteJSONRPCTransport{
		stream:               stream,
		sendRaw:              sendRaw,
		pending:              make(map[RequestID]chan JSONRPCResponse),
		notificationHandlers: make(map[string]NotificationHandler),
		requestHandlers:      make(map[string]RequestHandler),
		readCtx:              readCtx,
		readCancel:           readCancel,
	}
	t.wg.Add(1)
	go t.readLoop()
	return t
}

// NewSSEClientTransport opens one remote SSE transport and adapts it to the MCP client transport interface.
func NewSSEClientTransport(ctx context.Context, endpoint string, headers map[string]string) (Transport, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	trimmedEndpoint := strings.TrimSpace(endpoint)
	if trimmedEndpoint == "" {
		return nil, fmt.Errorf("missing sse endpoint")
	}
	if !strings.HasPrefix(trimmedEndpoint, "http://") && !strings.HasPrefix(trimmedEndpoint, "https://") {
		return nil, fmt.Errorf("invalid sse endpoint scheme: %s", trimmedEndpoint)
	}

	stream, err := remote.DialSSE(ctx, trimmedEndpoint, headers, nil, 0)
	if err != nil {
		return nil, err
	}

	httpClient := &http.Client{Timeout: 60 * time.Second}
	return newRemoteJSONRPCTransport(stream, func(sendCtx context.Context, data []byte) error {
		req, err := http.NewRequestWithContext(sendCtx, http.MethodPost, trimmedEndpoint, bytes.NewReader(data))
		if err != nil {
			return fmt.Errorf("mcp sse transport: build request: %w", err)
		}
		req.Header.Set("content-type", "application/json")
		for key, value := range headers {
			req.Header.Set(key, value)
		}
		resp, err := httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("mcp sse transport: send request: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("mcp sse transport: send request rejected: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
		}
		return nil
	}), nil
}

// NewWebSocketClientTransport opens one remote WebSocket transport and adapts it to the MCP client transport interface.
func NewWebSocketClientTransport(ctx context.Context, endpoint string, headers map[string]string) (Transport, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	trimmedEndpoint := strings.TrimSpace(endpoint)
	if trimmedEndpoint == "" {
		return nil, fmt.Errorf("missing websocket endpoint")
	}
	if !strings.HasPrefix(trimmedEndpoint, "ws://") && !strings.HasPrefix(trimmedEndpoint, "wss://") {
		return nil, fmt.Errorf("invalid websocket endpoint scheme: %s", trimmedEndpoint)
	}

	stream, err := remote.DialWebSocket(ctx, trimmedEndpoint, headers, nil)
	if err != nil {
		return nil, err
	}
	return newRemoteJSONRPCTransport(stream, func(sendCtx context.Context, data []byte) error {
		select {
		case <-sendCtx.Done():
			return sendCtx.Err()
		default:
		}
		return stream.Send(data)
	}), nil
}

// Send writes one JSON-RPC request through the remote transport and waits for the matching response.
func (t *remoteJSONRPCTransport) Send(ctx context.Context, req JSONRPCRequest) (*JSONRPCResponse, error) {
	if t == nil {
		return nil, fmt.Errorf("mcp remote transport: nil")
	}
	if t.closed.Load() {
		return nil, fmt.Errorf("mcp remote transport: closed")
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
		return nil, fmt.Errorf("mcp remote transport: marshal request: %w", err)
	}

	t.writeMu.Lock()
	err = t.sendRaw(ctx, data)
	t.writeMu.Unlock()
	if err != nil {
		return nil, err
	}

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

// SetNotificationHandler registers a callback for one incoming notification method.
func (t *remoteJSONRPCTransport) SetNotificationHandler(method string, handler NotificationHandler) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if handler == nil {
		delete(t.notificationHandlers, method)
		return
	}
	t.notificationHandlers[method] = handler
}

// SetRequestHandler registers a callback for one incoming server-initiated request method.
func (t *remoteJSONRPCTransport) SetRequestHandler(method string, handler RequestHandler) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if handler == nil {
		delete(t.requestHandlers, method)
		return
	}
	t.requestHandlers[method] = handler
}

// Close shuts down the remote transport and releases resources.
func (t *remoteJSONRPCTransport) Close() error {
	if t == nil {
		return nil
	}

	t.closeOnce.Do(func() {
		t.closed.Store(true)
		t.readCancel()
		if t.stream != nil {
			_ = t.stream.Close()
		}
	})
	t.wg.Wait()
	return nil
}

func (t *remoteJSONRPCTransport) readLoop() {
	defer t.wg.Done()

	for {
		event, err := t.stream.Recv(t.readCtx)
		if err != nil {
			if t.closed.Load() || err == context.Canceled || err == remote.ErrStreamClosed {
				return
			}
			logger.WarnCF("mcp", "remote transport read error", map[string]any{
				"error": err.Error(),
			})
			return
		}

		if len(event.Data) == 0 {
			continue
		}

		// Try incoming request first so server-initiated RPC calls do not get
		// mistaken for responses.
		var req JSONRPCRequest
		if err := json.Unmarshal(event.Data, &req); err == nil && req.Method != "" && req.ID != "" {
			t.handleRequest(req)
			continue
		}

		var resp JSONRPCResponse
		if err := json.Unmarshal(event.Data, &resp); err == nil && resp.ID != "" {
			t.mu.Lock()
			ch, ok := t.pending[resp.ID]
			t.mu.Unlock()
			if ok {
				ch <- resp
			} else {
				logger.DebugCF("mcp", "remote transport orphan response", map[string]any{
					"id": resp.ID,
				})
			}
			continue
		}

		var notif JSONRPCNotification
		if err := json.Unmarshal(event.Data, &notif); err == nil && notif.Method != "" {
			t.mu.Lock()
			handler := t.notificationHandlers[notif.Method]
			t.mu.Unlock()
			if handler != nil {
				go handler(notif)
			} else {
				logger.DebugCF("mcp", "remote transport unhandled notification", map[string]any{
					"method": notif.Method,
				})
			}
			continue
		}

		logger.DebugCF("mcp", "remote transport unparseable payload", map[string]any{
			"transport": event.Transport,
			"type":      event.Type,
			"payload":   string(event.Data),
		})
	}
}

func (t *remoteJSONRPCTransport) handleRequest(req JSONRPCRequest) {
	t.mu.Lock()
	handler := t.requestHandlers[req.Method]
	t.mu.Unlock()

	if handler == nil {
		t.writeResponse(JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &JSONRPCError{
				Code:    -32601,
				Message: fmt.Sprintf("mcp remote transport: unhandled request %q", req.Method),
			},
		})
		return
	}

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

	payload, err := json.Marshal(result)
	if err != nil {
		t.writeResponse(JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &JSONRPCError{
				Code:    -32603,
				Message: fmt.Sprintf("mcp remote transport: marshal request handler result: %v", err),
			},
		})
		return
	}

	t.writeResponse(JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  payload,
	})
}

func (t *remoteJSONRPCTransport) writeResponse(resp JSONRPCResponse) {
	data, err := json.Marshal(resp)
	if err != nil {
		logger.WarnCF("mcp", "remote transport marshal response failed", map[string]any{
			"id":    resp.ID,
			"error": err.Error(),
		})
		return
	}

	if err := t.sendRaw(context.Background(), data); err != nil {
		logger.WarnCF("mcp", "remote transport write response failed", map[string]any{
			"id":    resp.ID,
			"error": err.Error(),
		})
	}
}
