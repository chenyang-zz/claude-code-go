package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sheepzhao/claude-code-go/internal/core/event"
	"github.com/sheepzhao/claude-code-go/internal/runtime/approval"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// Message types over WebSocket.
const (
	msgTypeEvent   = "event"
	msgTypeInput   = "input"
	msgTypeLine    = "line"
	msgTypeApprovalPrompt = "approval_prompt"
	msgTypeApproval = "approval"
)

// WSMessage is the wire format for all WebSocket communication.
type WSMessage struct {
	Type     string      `json:"type"`
	Payload  any `json:"payload,omitempty"`
}

// eventPayload wraps an event.Event for JSON serialization.
type eventPayload struct {
	Type      event.Type `json:"type"`
	Timestamp time.Time  `json:"timestamp"`
	Payload   any `json:"payload,omitempty"`
}

// Renderer implements console.EventRenderer by forwarding events over WebSocket
// to one connected TUI client. It also provides an io.Reader for user input
// received from the TUI via WebSocket messages.
type Renderer struct {
	server   *http.Server
	upgrader websocket.Upgrader
	listener net.Listener
	port     int

	mu      sync.RWMutex
	conn    *websocket.Conn
	connCh  chan struct{} // closed when first (or next) TUI connects

	writeMu       sync.Mutex // serializes gorilla/websocket writes
	inputCh       chan string   // TUI input lines arrive here
	approvalRespCh chan approval.Response // TUI approval responses arrive here (buffer 1)
	done          chan struct{} // closed on Close
}

// NewRenderer creates a TUI renderer, starts listening on a random TCP port,
// and returns immediately. The caller must call WaitForConnection before
// sending events if it needs the TUI to be connected.
func NewRenderer() (*Renderer, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("tui listen: %w", err)
	}

	r := &Renderer{
		listener: listener,
		port:     listener.Addr().(*net.TCPAddr).Port,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		connCh:         make(chan struct{}),
		inputCh:        make(chan string, 64),
		approvalRespCh: make(chan approval.Response, 1),
		done:           make(chan struct{}),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", r.handleWS)
	r.server = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go r.server.Serve(listener) //nolint:errcheck

	logger.DebugCF("tui", "WebSocket server listening", map[string]any{"port": r.port})
	return r, nil
}

// Port returns the TCP port the WebSocket server is listening on.
func (r *Renderer) Port() int {
	return r.port
}

// WaitForConnection blocks until a TUI client connects, or until the context
// is cancelled. Returns nil on successful connection.
func (r *Renderer) WaitForConnection(ctx context.Context) error {
	r.mu.RLock()
	if r.conn != nil {
		r.mu.RUnlock()
		return nil
	}
	ch := r.connCh
	r.mu.RUnlock()

	select {
	case <-ch:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// RenderEvent implements console.EventRenderer by sending the event
// as a JSON message over WebSocket.
func (r *Renderer) RenderEvent(evt event.Event) error {
	msg := WSMessage{
		Type: msgTypeEvent,
		Payload: eventPayload{
			Type:      evt.Type,
			Timestamp: evt.Timestamp,
			Payload:   evt.Payload,
		},
	}
	return r.writeJSON(msg)
}

// RenderLine implements console.EventRenderer by sending the text
// as a "line" message over WebSocket.
func (r *Renderer) RenderLine(text string) error {
	msg := WSMessage{
		Type: msgTypeLine,
		Payload: map[string]string{
			"text": text,
		},
	}
	return r.writeJSON(msg)
}

// InputReader returns an io.Reader that yields one line of user input
// (with trailing newline) for each TUI "input" message received.
func (r *Renderer) InputReader() io.Reader {
	return &wsInputReader{ch: r.inputCh, done: r.done}
}

// Close shuts down the WebSocket server and all connections.
func (r *Renderer) Close() error {
	close(r.done)

	r.mu.Lock()
	if r.conn != nil {
		r.conn.Close() //nolint:errcheck
		r.conn = nil
	}
	r.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return r.server.Shutdown(ctx)
}

// writeJSON sends a message to the connected TUI, silently dropping
// the message when no client is connected.
func (r *Renderer) writeJSON(msg WSMessage) error {
	r.mu.RLock()
	conn := r.conn
	r.mu.RUnlock()

	if conn == nil {
		return nil
	}

	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	// Set a short write deadline so a stuck TUI doesn't block the engine.
	if err := conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return nil
	}
	return conn.WriteJSON(msg)
}

// handleWS upgrades an HTTP connection to WebSocket and manages
// the client lifecycle.
func (r *Renderer) handleWS(w http.ResponseWriter, req *http.Request) {
	conn, err := r.upgrader.Upgrade(w, req, nil)
	if err != nil {
		logger.WarnCF("tui", "WebSocket upgrade failed", map[string]any{"error": err.Error()})
		return
	}

	logger.DebugCF("tui", "TUI client connected", nil)

	// Disable Nagle's algorithm so small WebSocket frames (e.g. individual
	// message.delta events) are sent immediately rather than buffered by TCP.
	if tcp, ok := conn.UnderlyingConn().(*net.TCPConn); ok {
		tcp.SetNoDelay(true) //nolint:errcheck
	}

	r.mu.Lock()
	// Close any previous connection.
	if r.conn != nil {
		r.conn.Close() //nolint:errcheck
	}
	r.conn = conn
	r.mu.Unlock()

	// Signal waiters.
	select {
	case <-r.connCh:
		// Already signalled – reconnect scenario.
	default:
		close(r.connCh)
	}

	// Read messages from the TUI until the connection drops.
	defer func() {
		r.mu.Lock()
		if r.conn == conn {
			r.conn = nil
			// Reset connCh so WaitForConnection blocks again.
			r.connCh = make(chan struct{})
			// Unblock any waiting ApprovalPrompter with a denied response.
			select {
			case r.approvalRespCh <- approval.Response{Approved: false, Reason: "TUI disconnected"}:
			default:
			}
		}
		r.mu.Unlock()
		conn.Close() //nolint:errcheck
		logger.DebugCF("tui", "TUI client disconnected", nil)
	}()

	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			break
		}

		var msg WSMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			continue
		}

		switch msg.Type {
		case msgTypeInput:
			payload, ok := msg.Payload.(map[string]any)
			if !ok {
				continue
			}
			text, ok := payload["text"].(string)
			if !ok {
				continue
			}
			select {
			case r.inputCh <- text:
			default:
				// Input channel full; drop.
			}
		case msgTypeApproval:
			payload, ok := msg.Payload.(map[string]any)
			if !ok {
				continue
			}
			approved, ok := payload["approved"].(bool)
			if !ok {
				continue
			}
			select {
			case r.approvalRespCh <- approval.Response{Approved: approved}:
			default:
				// No one waiting for approval; drop.
			}
		}
	}
}

// wsInputReader converts WebSocket input messages into an io.Reader
// that yields lines of text.
type wsInputReader struct {
	ch   <-chan string
	done <-chan struct{}
	buf  string
}

func (r *wsInputReader) Read(p []byte) (int, error) {
	for r.buf == "" {
		select {
		case line, ok := <-r.ch:
			if !ok {
				return 0, io.EOF
			}
			r.buf = line + "\n"
		case <-r.done:
			return 0, io.EOF
		}
	}

	n := copy(p, r.buf)
	r.buf = r.buf[n:]
	return n, nil
}

// ApprovalPrompter implements approval.Prompter by sending prompts to the TUI
// over WebSocket and blocking for a response.
type ApprovalPrompter struct {
	r *Renderer
}

// NewApprovalPrompter creates a Prompter that sends approval requests to the
// connected TUI via WebSocket and waits for an approve/deny response.
func NewApprovalPrompter(r *Renderer) *ApprovalPrompter {
	return &ApprovalPrompter{r: r}
}

// Prompt sends an approval request to the TUI and blocks until the user
// responds or the context is cancelled.
func (p *ApprovalPrompter) Prompt(ctx context.Context, prompt approval.Prompt) (approval.Response, error) {
	// Drain any stale response from a previous cancelled prompt.
	select {
	case <-p.r.approvalRespCh:
	default:
	}

	// Check whether a TUI client is actually connected before sending and
	// blocking. Without this check, a disconnected TUI causes Prompt to
	// block until ctx expires or the Renderer shuts down, stalling the
	// guarded tool call.
	if !p.r.isConnected() {
		return approval.Response{Approved: false, Reason: "No TUI connected"}, nil
	}

	// Send the approval prompt to the TUI.
	p.r.writeJSON(WSMessage{
		Type: msgTypeApprovalPrompt,
		Payload: map[string]string{
			"title": prompt.Title,
			"body":  prompt.Body,
		},
	})

	// Wait for a response or context cancellation.
	select {
	case resp := <-p.r.approvalRespCh:
		return resp, nil
	case <-ctx.Done():
		return approval.Response{}, ctx.Err()
	case <-p.r.done:
		return approval.Response{Approved: false, Reason: "TUI closed"}, nil
	}
}

// isConnected reports whether a TUI WebSocket client is currently connected.
func (r *Renderer) isConnected() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.conn != nil
}
