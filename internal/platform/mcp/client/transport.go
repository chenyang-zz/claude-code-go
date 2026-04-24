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
	// Close shuts down the transport and releases resources.
	Close() error
}

// StdioClientTransport runs an MCP server as a subprocess and communicates
// over stdin/stdout using line-delimited JSON-RPC 2.0 messages.
type StdioClientTransport struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stdout  io.ReadCloser
	stderr  io.ReadCloser
	pending map[RequestID]chan JSONRPCResponse
	mu      sync.Mutex
	nextID  atomic.Int64
	reader  *bufio.Scanner
	closeCh chan struct{}
	closed  atomic.Bool
	wg      sync.WaitGroup
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
		cmd:     cmd,
		stdin:   stdin,
		stdout:  stdout,
		stderr:  stderr,
		pending: make(map[RequestID]chan JSONRPCResponse),
		reader:  bufio.NewScanner(stdout),
		closeCh: make(chan struct{}),
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
	if _, err := t.stdin.Write(data); err != nil {
		return nil, fmt.Errorf("mcp stdio transport: write request: %w", err)
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

		// Try response first; fall back to notification.
		var resp JSONRPCResponse
		if err := json.Unmarshal(line, &resp); err != nil {
			logger.DebugCF("mcp", "stdio transport unparseable line", map[string]any{
				"line":  string(line),
				"error": err.Error(),
			})
			continue
		}

		// Empty ID means a notification; drop it for now.
		if resp.ID == "" {
			continue
		}

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
	}
}

// NextID returns a monotonically increasing request ID.
func (t *StdioClientTransport) NextID() RequestID {
	return RequestID(fmt.Sprintf("%d", t.nextID.Add(1)))
}
