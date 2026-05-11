package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
	"github.com/sheepzhao/claude-code-go/pkg/sdk"
)

// DirectConnectCallbacks receives events from the direct-connect session.
type DirectConnectCallbacks struct {
	// OnMessage is called for each SDK message received from the session.
	OnMessage func(data []byte)
	// OnPermissionRequest is called when a permission request arrives.
	OnPermissionRequest func(req *sdk.ControlPermissionRequest, requestID string)
	// OnConnected is called when the WebSocket connection is established.
	OnConnected func()
	// OnDisconnected is called when the WebSocket connection is lost.
	OnDisconnected func()
	// OnError is called on WebSocket errors.
	OnError func(err error)
}

// DirectConnectSessionManager manages one WebSocket connection to a direct-connect session.
type DirectConnectSessionManager struct {
	config    DirectConnectConfig
	callbacks DirectConnectCallbacks
	conn      *websocket.Conn
	cancel    chan struct{}
	closeOnce sync.Once
	closed    atomic.Bool
	mu        sync.Mutex
}

// NewDirectConnectSessionManager creates a new session manager.
func NewDirectConnectSessionManager(config DirectConnectConfig, callbacks DirectConnectCallbacks) *DirectConnectSessionManager {
	return &DirectConnectSessionManager{
		config:    config,
		callbacks: callbacks,
		cancel:    make(chan struct{}),
	}
}

// Connect opens the WebSocket connection and starts the message read loop.
func (m *DirectConnectSessionManager) Connect() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed.Load() {
		return fmt.Errorf("session manager is closed")
	}

	headers := http.Header{}
	if m.config.AuthToken != "" {
		headers.Set("authorization", "Bearer "+m.config.AuthToken)
	}

	logger.DebugCF("server_manager", "connecting to session", map[string]any{
		"ws_url":     m.config.WsURL,
		"session_id": m.config.SessionID,
	})

	conn, resp, err := websocket.DefaultDialer.Dial(m.config.WsURL, headers)
	if err != nil {
		status := 0
		if resp != nil {
			status = resp.StatusCode
		}
		return fmt.Errorf("connect to session %s: %w (status=%d)", m.config.SessionID, err, status)
	}
	m.conn = conn

	if m.callbacks.OnConnected != nil {
		m.callbacks.OnConnected()
	}

	go m.readLoop()
	return nil
}

// readLoop reads messages from the WebSocket and dispatches them.
func (m *DirectConnectSessionManager) readLoop() {
	defer func() {
		m.mu.Lock()
		if m.conn != nil {
			_ = m.conn.Close()
			m.conn = nil
		}
		m.mu.Unlock()
	}()

	for {
		select {
		case <-m.cancel:
			return
		default:
		}

		_, payload, err := m.conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				return
			}
			select {
			case <-m.cancel:
				return
			default:
			}
			if m.callbacks.OnError != nil {
				m.callbacks.OnError(fmt.Errorf("read websocket message: %w", err))
			}
			return
		}

		m.handleMessage(payload)
	}
}

// handleMessage parses a JSON lines message and routes it to the appropriate callback.
func (m *DirectConnectSessionManager) handleMessage(data []byte) {
	lines := splitLines(data)
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}

		var envelope struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(line, &envelope); err != nil {
			continue
		}

		switch envelope.Type {
		case "control_request":
			m.handleControlRequest(line)
		case "control_cancel_request":
			// Cancel requests are acknowledged and dropped — no callback needed.
		case "control_response", "keep_alive", "streamlined_text",
			"streamlined_tool_use_summary":
			continue
		default:
			if envelope.Type == "system" {
				var sys struct {
					Subtype string `json:"subtype"`
				}
				if json.Unmarshal(line, &sys) == nil && sys.Subtype == "post_turn_summary" {
					continue
				}
			}
			if m.callbacks.OnMessage != nil {
				m.callbacks.OnMessage(line)
			}
		}
	}
}

// handleControlRequest processes a control_request message.
func (m *DirectConnectSessionManager) handleControlRequest(data []byte) {
	var req sdk.ControlRequest
	if err := json.Unmarshal(data, &req); err != nil {
		logger.WarnCF("server_manager", "failed to unmarshal control request", map[string]any{
			"error": err.Error(),
		})
		return
	}

	var inner struct {
		Subtype string `json:"subtype"`
	}
	if err := json.Unmarshal(req.Request, &inner); err != nil {
		logger.WarnCF("server_manager", "failed to unmarshal control request inner", map[string]any{
			"error": err.Error(),
		})
		return
	}

	if inner.Subtype == "can_use_tool" {
		var permReq sdk.ControlPermissionRequest
		if err := json.Unmarshal(req.Request, &permReq); err != nil {
			logger.WarnCF("server_manager", "failed to unmarshal permission request", map[string]any{
				"error": err.Error(),
			})
			return
		}
		if m.callbacks.OnPermissionRequest != nil {
			m.callbacks.OnPermissionRequest(&permReq, req.RequestID)
		}
	} else {
		logger.DebugCF("server_manager", "unsupported control request subtype", map[string]any{
			"subtype": inner.Subtype,
		})
		_ = m.sendErrorResponse(req.RequestID,
			fmt.Sprintf("unsupported control request subtype: %s", inner.Subtype))
	}
}

// SendMessage sends a user message over the WebSocket connection.
func (m *DirectConnectSessionManager) SendMessage(content json.RawMessage) bool {
	m.mu.Lock()
	conn := m.conn
	m.mu.Unlock()

	if conn == nil {
		return false
	}

	message := map[string]any{
		"type": "user",
		"message": map[string]any{
			"role":    "user",
			"content": content,
		},
		"parent_tool_use_id": nil,
		"session_id":         "",
	}

	payload, err := json.Marshal(message)
	if err != nil {
		logger.WarnCF("server_manager", "failed to marshal user message", map[string]any{
			"error": err.Error(),
		})
		return false
	}

	if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
		logger.WarnCF("server_manager", "failed to send message", map[string]any{
			"error": err.Error(),
		})
		return false
	}
	return true
}

// RespondToPermissionRequest sends a control response for a pending permission request.
func (m *DirectConnectSessionManager) RespondToPermissionRequest(requestID string, result sdk.PermissionResponse) error {
	m.mu.Lock()
	conn := m.conn
	m.mu.Unlock()

	if conn == nil {
		return fmt.Errorf("not connected")
	}

	responseData := map[string]any{
		"behavior": result.Behavior,
	}
	if result.Behavior == "allow" {
		responseData["updatedInput"] = result.UpdatedInput
	} else {
		responseData["message"] = result.Message
	}

	resp := sdk.ControlResponse{
		Type: "control_response",
		Response: sdk.ControlResponseInner{
			Subtype:   "success",
			RequestID: requestID,
			Response:  responseData,
		},
	}

	payload, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("marshal control response: %w", err)
	}

	return conn.WriteMessage(websocket.TextMessage, payload)
}

// SendInterrupt sends an interrupt control request to cancel the current request.
func (m *DirectConnectSessionManager) SendInterrupt() error {
	m.mu.Lock()
	conn := m.conn
	m.mu.Unlock()

	if conn == nil {
		return fmt.Errorf("not connected")
	}

	inner, err := json.Marshal(sdk.ControlInterruptRequest{Subtype: "interrupt"})
	if err != nil {
		return fmt.Errorf("marshal interrupt inner: %w", err)
	}

	req := sdk.ControlRequest{
		Type:      "control_request",
		RequestID: uuid.NewString(),
		Request:   inner,
	}

	payload, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal interrupt request: %w", err)
	}

	return conn.WriteMessage(websocket.TextMessage, payload)
}

// sendErrorResponse sends an error response for an unknown control request.
func (m *DirectConnectSessionManager) sendErrorResponse(requestID, errMsg string) error {
	m.mu.Lock()
	conn := m.conn
	m.mu.Unlock()

	if conn == nil {
		return fmt.Errorf("not connected")
	}

	resp := sdk.ControlResponse{
		Type: "control_response",
		Response: sdk.ControlResponseInner{
			Subtype:   "error",
			RequestID: requestID,
			Error:     errMsg,
		},
	}

	payload, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("marshal error response: %w", err)
	}

	return conn.WriteMessage(websocket.TextMessage, payload)
}

// Disconnect closes the WebSocket connection and stops the read loop.
func (m *DirectConnectSessionManager) Disconnect() {
	m.closeOnce.Do(func() {
		m.closed.Store(true)
		close(m.cancel)
		m.mu.Lock()
		if m.conn != nil {
			_ = m.conn.Close()
			m.conn = nil
		}
		m.mu.Unlock()
	})
}

// IsConnected returns true if the WebSocket is currently connected.
func (m *DirectConnectSessionManager) IsConnected() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.conn != nil
}

// splitLines splits raw bytes into lines, trimming whitespace.
func splitLines(data []byte) [][]byte {
	var lines [][]byte
	start := 0
	for i, b := range data {
		if b == '\n' {
			if i > start {
				lines = append(lines, data[start:i])
			}
			start = i + 1
		}
	}
	if start < len(data) {
		lines = append(lines, data[start:])
	}
	return lines
}
