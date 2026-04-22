package remote

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// WebSocketClient implements one receive-only WebSocket event stream.
type WebSocketClient struct {
	// conn stores the active WebSocket connection.
	conn *websocket.Conn
	// cancel stops the internal read loop.
	cancel context.CancelFunc
	// streamCh carries normalized ws messages and terminal read errors.
	streamCh chan streamResult
	// closeOnce guarantees close side effects run once.
	closeOnce sync.Once
}

// DialWebSocket opens one ws/wss connection and starts a background message reader.
func DialWebSocket(
	ctx context.Context,
	endpoint string,
	headers map[string]string,
	dialer *websocket.Dialer,
) (*WebSocketClient, error) {
	trimmedEndpoint := strings.TrimSpace(endpoint)
	if trimmedEndpoint == "" {
		return nil, fmt.Errorf("missing websocket endpoint")
	}
	if !strings.HasPrefix(trimmedEndpoint, "ws://") && !strings.HasPrefix(trimmedEndpoint, "wss://") {
		return nil, fmt.Errorf("invalid websocket endpoint scheme: %s", trimmedEndpoint)
	}

	wsDialer := dialer
	if wsDialer == nil {
		wsDialer = websocket.DefaultDialer
	}

	reqHeader := http.Header{}
	for key, value := range headers {
		reqHeader.Set(key, value)
	}

	logger.DebugCF("remote_ws", "opening websocket stream", map[string]any{
		"endpoint": trimmedEndpoint,
	})

	conn, resp, err := wsDialer.DialContext(ctx, trimmedEndpoint, reqHeader)
	if err != nil {
		status := 0
		if resp != nil {
			status = resp.StatusCode
		}
		return nil, fmt.Errorf("connect websocket stream: %w (status=%d)", err, status)
	}

	streamCtx, cancel := context.WithCancel(context.Background())
	c := &WebSocketClient{
		conn:     conn,
		cancel:   cancel,
		streamCh: make(chan streamResult, 16),
	}
	go c.readLoop(streamCtx)

	logger.DebugCF("remote_ws", "websocket stream connected", map[string]any{
		"endpoint": trimmedEndpoint,
	})

	return c, nil
}

// Recv returns one WebSocket message converted into one normalized remote event.
func (c *WebSocketClient) Recv(ctx context.Context) (Event, error) {
	if c == nil {
		return Event{}, ErrStreamClosed
	}

	select {
	case <-ctx.Done():
		return Event{}, ctx.Err()
	case result, ok := <-c.streamCh:
		if !ok {
			return Event{}, ErrStreamClosed
		}
		if result.err != nil {
			return Event{}, result.err
		}
		return result.event, nil
	}
}

// Close stops the reader and closes the underlying WebSocket connection.
func (c *WebSocketClient) Close() error {
	if c == nil {
		return nil
	}

	c.closeOnce.Do(func() {
		c.cancel()
		if c.conn != nil {
			_ = c.conn.Close()
		}
	})
	return nil
}

func (c *WebSocketClient) readLoop(ctx context.Context) {
	defer close(c.streamCh)
	defer func() {
		if c.conn != nil {
			_ = c.conn.Close()
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		messageType, payload, err := c.conn.ReadMessage()
		if err != nil {
			// Normal closure maps to stream closed for caller consumption.
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				return
			}
			select {
			case <-ctx.Done():
				return
			case c.streamCh <- streamResult{
				err: fmt.Errorf("read websocket stream: %w", err),
			}:
			}
			return
		}

		eventType := websocketMessageTypeName(messageType)
		select {
		case <-ctx.Done():
			return
		case c.streamCh <- streamResult{
			event: Event{
				Transport: TransportWebSocket,
				Type:      eventType,
				Data:      payload,
			},
		}:
		}
	}
}

// websocketMessageTypeName normalizes gorilla message types into stable string labels.
func websocketMessageTypeName(messageType int) string {
	switch messageType {
	case websocket.TextMessage:
		return "text"
	case websocket.BinaryMessage:
		return "binary"
	case websocket.PingMessage:
		return "ping"
	case websocket.PongMessage:
		return "pong"
	case websocket.CloseMessage:
		return "close"
	default:
		return "unknown"
	}
}
