package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// httpClientTransport implements the minimal MCP HTTP request/response path.
type httpClientTransport struct {
	endpoint string
	headers  map[string]string
	client   *http.Client
	closed   atomic.Bool
	nextID   atomic.Int64
}

// NewHTTPClientTransport opens a minimal HTTP transport for one MCP server.
func NewHTTPClientTransport(ctx context.Context, endpoint string, headers map[string]string) (Transport, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	_ = ctx

	trimmedEndpoint := strings.TrimSpace(endpoint)
	if trimmedEndpoint == "" {
		return nil, fmt.Errorf("missing http endpoint")
	}
	if !strings.HasPrefix(trimmedEndpoint, "http://") && !strings.HasPrefix(trimmedEndpoint, "https://") {
		return nil, fmt.Errorf("invalid http endpoint scheme: %s", trimmedEndpoint)
	}

	copiedHeaders := make(map[string]string, len(headers))
	for key, value := range headers {
		copiedHeaders[key] = value
	}

	return &httpClientTransport{
		endpoint: trimmedEndpoint,
		headers:  copiedHeaders,
		client:   &http.Client{Timeout: 60 * time.Second},
	}, nil
}

// Send posts one JSON-RPC request and waits for the matching JSON-RPC response.
func (t *httpClientTransport) Send(ctx context.Context, req JSONRPCRequest) (*JSONRPCResponse, error) {
	if t == nil {
		return nil, fmt.Errorf("mcp http transport: nil")
	}
	if t.closed.Load() {
		return nil, fmt.Errorf("mcp http transport: closed")
	}

	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("mcp http transport: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, t.endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("mcp http transport: build request: %w", err)
	}
	httpReq.Header.Set("content-type", "application/json")
	httpReq.Header.Set("accept", "application/json")
	for key, value := range t.headers {
		httpReq.Header.Set(key, value)
	}

	resp, err := t.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("mcp http transport: send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("mcp http transport: read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		logger.WarnCF("mcp", "http transport rejected response", map[string]any{
			"endpoint":    t.endpoint,
			"status_code": resp.StatusCode,
			"body":        strings.TrimSpace(string(body)),
		})
		return nil, fmt.Errorf("mcp http transport: request rejected: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("mcp http transport: empty response body")
	}

	var rpcResp JSONRPCResponse
	if err := json.Unmarshal(body, &rpcResp); err != nil {
		logger.WarnCF("mcp", "http transport decode response failed", map[string]any{
			"endpoint": t.endpoint,
			"error":    err.Error(),
			"body":     strings.TrimSpace(string(body)),
		})
		return nil, fmt.Errorf("mcp http transport: decode response: %w", err)
	}
	if rpcResp.ID == "" {
		return nil, fmt.Errorf("mcp http transport: missing response id")
	}
	return &rpcResp, nil
}

// SetNotificationHandler records the handler for completeness; the minimal HTTP transport does not stream inbound events yet.
func (t *httpClientTransport) SetNotificationHandler(method string, handler NotificationHandler) {
	_ = method
	_ = handler
}

// SetRequestHandler records the handler for completeness; the minimal HTTP transport does not stream inbound events yet.
func (t *httpClientTransport) SetRequestHandler(method string, handler RequestHandler) {
	_ = method
	_ = handler
}

// Close marks the transport closed. The minimal HTTP transport does not own a persistent stream.
func (t *httpClientTransport) Close() error {
	if t == nil {
		return nil
	}
	t.closed.Store(true)
	return nil
}

// NextID returns a monotonically increasing request ID for HTTP requests.
func (t *httpClientTransport) NextID() RequestID {
	return RequestID(fmt.Sprintf("%d", t.nextID.Add(1)))
}
