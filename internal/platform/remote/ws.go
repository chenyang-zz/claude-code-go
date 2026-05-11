package remote

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// DefaultPingInterval is the interval between WebSocket keepalive pings.
const DefaultPingInterval = 30 * time.Second

// WebSocketClient implements one bidirectional WebSocket event stream.
type WebSocketClient struct {
	// conn stores the active WebSocket connection.
	conn *websocket.Conn
	// cancel stops the internal read loop.
	cancel context.CancelFunc
	// streamCh carries normalized ws messages and terminal read errors.
	streamCh chan streamResult
	// closeOnce guarantees close side effects run once.
	closeOnce sync.Once
	// closed is set to true after Close() is called.
	closed atomic.Bool
	// pingInterval is the duration between keepalive pings. 0 means disabled.
	pingInterval time.Duration
	// pingTicker fires at pingInterval to send keepalive pings.
	pingTicker *time.Ticker
	// stopPing signals the ping goroutine to stop.
	stopPing chan struct{}
}

// WebSocketOption configures a WebSocketClient.
type WebSocketOption func(*WebSocketClient)

// WithPingInterval sets the WebSocket keepalive ping interval.
// When set, the client sends periodic pings and expects pong responses.
// A read deadline is set to 3x the interval so stale connections are
// detected and the read loop exits, allowing the caller to reconnect.
func WithPingInterval(interval time.Duration) WebSocketOption {
	return func(c *WebSocketClient) {
		c.pingInterval = interval
	}
}

// DialWebSocket opens one ws/wss connection and starts a background message reader.
func DialWebSocket(
	ctx context.Context,
	endpoint string,
	headers map[string]string,
	dialer *websocket.Dialer,
	opts ...WebSocketOption,
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
		stopPing: make(chan struct{}),
	}
	for _, opt := range opts {
		opt(c)
	}

	// Configure ping/pong keepalive if enabled.
	if c.pingInterval > 0 {
		readDeadline := 3 * c.pingInterval
		if err := conn.SetReadDeadline(time.Now().Add(readDeadline)); err != nil {
			cancel()
			_ = conn.Close()
			return nil, fmt.Errorf("set read deadline: %w", err)
		}
		conn.SetPongHandler(func(string) error {
			return conn.SetReadDeadline(time.Now().Add(readDeadline))
		})
		c.pingTicker = time.NewTicker(c.pingInterval)
		go c.pingLoop(readDeadline)
	}

	go c.readLoop(streamCtx)

	logger.DebugCF("remote_ws", "websocket stream connected", map[string]any{
		"endpoint":     trimmedEndpoint,
		"ping_seconds": c.pingInterval.Seconds(),
	})

	return c, nil
}

// pingLoop sends periodic ping frames to keep the connection alive.
func (c *WebSocketClient) pingLoop(readDeadline time.Duration) {
	defer c.pingTicker.Stop()
	for {
		select {
		case <-c.stopPing:
			return
		case <-c.pingTicker.C:
			if c.closed.Load() {
				return
			}
			if err := c.conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(readDeadline/3)); err != nil {
				logger.DebugCF("remote_ws", "ping write failed", map[string]any{
					"error": err.Error(),
				})
				return
			}
		}
	}
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
		c.closed.Store(true)
		// Stop ping loop.
		select {
		case c.stopPing <- struct{}{}:
		default:
		}
		c.cancel()
		if c.conn != nil {
			_ = c.conn.Close()
		}
	})
	return nil
}

// Send writes one text message to the WebSocket connection.
func (c *WebSocketClient) Send(data []byte) error {
	if c == nil {
		return fmt.Errorf("websocket client is nil")
	}
	if c.closed.Load() {
		return ErrStreamClosed
	}
	if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
		return fmt.Errorf("write websocket message: %w", err)
	}
	return nil
}

// GetLastSequenceNum returns 0 as the base WebSocket client does not track
// sequence numbers. Implemented for the SeqNumProvider interface used by
// ResilientEventStream.
func (c *WebSocketClient) GetLastSequenceNum() int64 {
	return 0
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

// websocketMessageTypeName normalises gorilla message types into stable string labels.
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
