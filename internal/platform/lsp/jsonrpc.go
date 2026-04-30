package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync/atomic"
)

// JSON-RPC 2.0 protocol constants.
const jsonrpcVersion = "2.0"

// nextRequestID provides monotonically increasing request IDs.
var nextRequestID atomic.Int64

func init() {
	nextRequestID.Store(1)
}

// Request represents a JSON-RPC 2.0 request message.
type Request struct {
	JSONRPC string         `json:"jsonrpc"`
	ID      int64          `json:"id"`
	Method  string         `json:"method"`
	Params  map[string]any `json:"params,omitempty"`
}

// NewRequest creates a new JSON-RPC request with a unique ID.
func NewRequest(method string, params map[string]any) Request {
	return Request{
		JSONRPC: jsonrpcVersion,
		ID:      nextRequestID.Add(1),
		Method:  method,
		Params:  params,
	}
}

// Response represents a JSON-RPC 2.0 response message.
// Result is stored as RawMessage to support both object and array results.
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// RPCError represents a JSON-RPC 2.0 error object.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// Notification represents a JSON-RPC 2.0 notification (no id field).
type Notification struct {
	JSONRPC string         `json:"jsonrpc"`
	Method  string         `json:"method"`
	Params  map[string]any `json:"params,omitempty"`
}

// NewNotification creates a new JSON-RPC notification.
func NewNotification(method string, params map[string]any) Notification {
	return Notification{
		JSONRPC: jsonrpcVersion,
		Method:  method,
		Params:  params,
	}
}

// WriteMessage encodes msg as a Content-Length-prefixed JSON-RPC frame and writes it to w.
func WriteMessage(w io.Writer, msg any) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("jsonrpc marshal: %w", err)
	}
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(data))
	if _, err := io.WriteString(w, header); err != nil {
		return fmt.Errorf("jsonrpc write header: %w", err)
	}
	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("jsonrpc write body: %w", err)
	}
	return nil
}

// ReadMessage reads a Content-Length-prefixed JSON-RPC frame from r into dst.
// dst must be a pointer to a struct that can be unmarshaled from JSON.
func ReadMessage(r *bufio.Reader, dst any) error {
	header, err := r.ReadString('\n')
	if err != nil {
		return fmt.Errorf("jsonrpc read header: %w", err)
	}

	header = strings.TrimSpace(header)
	if !strings.HasPrefix(header, "Content-Length: ") {
		return fmt.Errorf("jsonrpc: malformed header %q", header)
	}

	length, err := strconv.Atoi(strings.TrimPrefix(header, "Content-Length: "))
	if err != nil {
		return fmt.Errorf("jsonrpc parse content length: %w", err)
	}

	// Discard the empty line separating header from body.
	if _, err := r.ReadString('\n'); err != nil {
		return fmt.Errorf("jsonrpc read separator: %w", err)
	}

	body := make([]byte, length)
	if _, err := io.ReadFull(r, body); err != nil {
		return fmt.Errorf("jsonrpc read body: %w", err)
	}

	if err := json.Unmarshal(body, dst); err != nil {
		return fmt.Errorf("jsonrpc unmarshal: %w", err)
	}

	return nil
}
