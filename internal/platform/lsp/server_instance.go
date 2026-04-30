package lsp

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// LSP_ERROR_CONTENT_MODIFIED is the LSP error code for "content modified"
// transient errors. Servers like rust-analyzer send this while indexing, and
// clients should retry silently per the LSP specification.
const lspErrorContentModified = -32801

// maxRetriesForTransientErrors is the maximum number of retries for transient
// LSP errors like "content modified".
const maxRetriesForTransientErrors = 3

// retryBaseDelayMS is the base delay in milliseconds for exponential backoff
// on transient errors.
const retryBaseDelayMS = 500

// ServerInstance manages the complete lifecycle of a single LSP server process.
// It wraps a Client with a formal state machine, health tracking, restart
// limiting, and transient error retry for outgoing requests.
type ServerInstance struct {
	// Name is the unique identifier for this server instance.
	Name string
	// Config holds the server launch configuration.
	Config ServerConfig

	mu           sync.RWMutex
	state        ServerState
	startTime    time.Time
	lastError    error
	restartCount int
	client       *Client
}

// NewServerInstance creates a new LSP server instance with the given name and
// configuration. The server is in Stopped state until Start is called.
func NewServerInstance(name string, config ServerConfig) *ServerInstance {
	return &ServerInstance{
		Name:   name,
		Config: config,
		state:  ServerStateStopped,
		client: NewClient(),
	}
}

// State returns the current server state.
func (s *ServerInstance) State() ServerState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

// StartTime returns the time the server last successfully started, or zero if
// it has never been started.
func (s *ServerInstance) StartTime() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.startTime
}

// LastError returns the most recent error encountered, or nil.
func (s *ServerInstance) LastError() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastError
}

// RestartCount returns the number of times Restart has been called.
func (s *ServerInstance) RestartCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.restartCount
}

// IsInitialized returns whether the underlying client has completed the
// initialize handshake.
func (s *ServerInstance) IsInitialized() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.client.isInitialized
}

// Capabilities returns the server capabilities discovered during initialize,
// or nil if the server has not been initialized.
func (s *ServerInstance) Capabilities() *ServerCapabilities {
	return s.client.Capabilities()
}

// Start launches the LSP server process and performs the initialize handshake.
// If the server is already running or starting, Start returns immediately.
// If the server has previously failed to start, it may be retried.
func (s *ServerInstance) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state == ServerStateRunning || s.state == ServerStateStarting {
		return nil
	}

	s.state = ServerStateStarting
	logger.DebugCF("lsp.server_instance", "starting server", map[string]any{
		"server":  s.Name,
		"command": s.Config.Command,
	})

	if err := s.client.Start(s.Config.Command, s.Config.Args...); err != nil {
		s.state = ServerStateError
		s.lastError = fmt.Errorf("start %q: %w", s.Name, err)
		logger.DebugCF("lsp.server_instance", "start failed", map[string]any{
			"server": s.Name,
			"error":  err.Error(),
		})
		return s.lastError
	}

	cwd, _ := os.Getwd()
	rootURI := "file://" + cwd

	if _, err := s.client.Initialize(rootURI); err != nil {
		s.state = ServerStateError
		s.lastError = fmt.Errorf("initialize %q: %w", s.Name, err)
		s.client.Shutdown()
		logger.DebugCF("lsp.server_instance", "initialize failed", map[string]any{
			"server": s.Name,
			"error":  err.Error(),
		})
		return s.lastError
	}

	s.state = ServerStateRunning
	s.startTime = time.Now()
	s.lastError = nil

	logger.DebugCF("lsp.server_instance", "server started", map[string]any{
		"server": s.Name,
	})
	return nil
}

// Stop gracefully shuts down the LSP server. If the server is already stopped
// or stopping, Stop returns immediately.
func (s *ServerInstance) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state == ServerStateStopped {
		return nil
	}

	logger.DebugCF("lsp.server_instance", "stopping server", map[string]any{
		"server": s.Name,
	})

	if err := s.client.Shutdown(); err != nil {
		s.state = ServerStateError
		s.lastError = fmt.Errorf("stop %q: %w", s.Name, err)
		logger.DebugCF("lsp.server_instance", "stop failed", map[string]any{
			"server": s.Name,
			"error":  err.Error(),
		})
		return s.lastError
	}

	s.state = ServerStateStopped
	logger.DebugCF("lsp.server_instance", "server stopped", map[string]any{
		"server": s.Name,
	})
	return nil
}

// Restart stops the server and starts it again. It increments the restart
// count and returns an error if the server fails to stop or start.
func (s *ServerInstance) Restart() error {
	if err := s.Stop(); err != nil {
		return fmt.Errorf("failed to stop %q during restart: %w", s.Name, err)
	}

	s.mu.Lock()
	s.restartCount++
	count := s.restartCount
	s.mu.Unlock()

	if err := s.Start(); err != nil {
		return fmt.Errorf("failed to start %q during restart (attempt %d): %w", s.Name, count, err)
	}

	return nil
}

// IsHealthy returns true if the server is running and has completed
// initialization, indicating it is ready to handle requests.
func (s *ServerInstance) IsHealthy() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state == ServerStateRunning && s.client.isInitialized
}

// SendRequest sends an LSP request to the server with automatic retry for
// transient "content modified" errors (code -32801). It returns the raw JSON
// result. An error is returned if the server is not healthy or the request
// fails after all retries.
func (s *ServerInstance) SendRequest(method string, params any) (json.RawMessage, error) {
	if !s.IsHealthy() {
		s.mu.RLock()
		err := fmt.Errorf("cannot send request to LSP server %q: server is %s", s.Name, s.state)
		if s.lastError != nil {
			err = fmt.Errorf("cannot send request to LSP server %q: server is %s, last error: %v", s.Name, s.state, s.lastError)
		}
		s.mu.RUnlock()
		return nil, err
	}

	var lastErr error
	for attempt := 0; attempt <= maxRetriesForTransientErrors; attempt++ {
		result, err := s.client.SendRequest(method, params)
		if err == nil {
			return result, nil
		}

		lastErr = err

		// Check for transient "content modified" error
		if isContentModifiedError(err) && attempt < maxRetriesForTransientErrors {
			delay := time.Duration(retryBaseDelayMS*(1<<attempt)) * time.Millisecond
			logger.DebugCF("lsp.server_instance", "transient error, retrying", map[string]any{
				"server":  s.Name,
				"method":  method,
				"attempt": attempt + 1,
				"delay":   delay.String(),
			})
			time.Sleep(delay)
			continue
		}

		break
	}

	return nil, fmt.Errorf("LSP request %q failed for server %q: %w", method, s.Name, lastErr)
}

// SendNotification sends a fire-and-forget notification to the LSP server.
// An error is returned if the server is not healthy or the send fails.
func (s *ServerInstance) SendNotification(method string, params any) error {
	if !s.IsHealthy() {
		s.mu.RLock()
		defer s.mu.RUnlock()
		return fmt.Errorf("cannot send notification to LSP server %q: server is %s", s.Name, s.state)
	}

	return s.client.SendNotification(method, params)
}

// OnNotification registers a handler for incoming notifications from the
// server with the given method name.
func (s *ServerInstance) OnNotification(method string, handler NotificationHandler) {
	s.client.OnNotification(method, handler)
}

// RequestHandler is a callback that handles an incoming LSP request from the
// server and returns a result. It is the Go equivalent of TS onRequest.
type RequestHandler func(rawParams json.RawMessage) (json.RawMessage, error)

// OnRequest registers a handler for incoming requests from the LSP server.
// Some LSP servers send requests TO the client (e.g., workspace/configuration).
// This allows registering handlers for such requests.
//
// NOTE: The current Go Client does not yet dispatch server-to-client requests
// in readLoop. This method registers the handler for future use.
func (s *ServerInstance) OnRequest(method string, handler RequestHandler) {
	// Registered for future use; Client.readLoop does not yet dispatch
	// server-to-client requests.
	_ = method
	_ = handler
}

// isContentModifiedError checks whether an error is an LSP "content modified"
// transient error (code -32801).
func isContentModifiedError(err error) bool {
	// Check for LSP error message with code -32801.
	// The error format from Client.SendRequest is: "lsp: <message> (code=<n>)"
	if err == nil {
		return false
	}
	errStr := err.Error()
	return contains(errStr, fmt.Sprintf("code=%d", lspErrorContentModified))
}

// contains returns true if s contains substr.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
