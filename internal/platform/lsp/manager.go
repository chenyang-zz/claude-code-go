package lsp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// ServerState represents the lifecycle state of an LSP server.
type ServerState int

const (
	ServerStateStopped ServerState = iota
	ServerStateStarting
	ServerStateRunning
	ServerStateError
)

// String returns a human-readable representation of the server state.
func (s ServerState) String() string {
	switch s {
	case ServerStateStopped:
		return "stopped"
	case ServerStateStarting:
		return "starting"
	case ServerStateRunning:
		return "running"
	case ServerStateError:
		return "error"
	default:
		return "unknown"
	}
}

// ServerConfig holds the configuration for launching an LSP server.
type ServerConfig struct {
	Command string            // LSP server binary path
	Args    []string          // CLI arguments for the server
	Env     map[string]string // Additional environment variables
}

// serverEntry holds the runtime state of a registered LSP server.
type serverEntry struct {
	Name   string
	Config ServerConfig
	Client *Client
	State  ServerState
}

// Manager manages multiple LSP server instances and routes requests
// based on file extensions. It provides lazy server startup and
// file open/close tracking.
type Manager struct {
	mu       sync.Mutex
	servers  map[string]*serverEntry   // server name → entry
	extMap   map[string]string         // file extension → server name
	openFiles map[string]string        // file URI → server name
}

// NewManager creates a new LSP server manager.
func NewManager() *Manager {
	return &Manager{
		servers:   make(map[string]*serverEntry),
		extMap:    make(map[string]string),
		openFiles: make(map[string]string),
	}
}

// RegisterServer registers an LSP server configuration for a set of file extensions.
// The server is not started until the first request is made.
func (m *Manager) RegisterServer(name string, config ServerConfig, extensions []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.servers[name]; exists {
		return fmt.Errorf("lsp manager: server %q already registered", name)
	}

	m.servers[name] = &serverEntry{
		Name:   name,
		Config: config,
		Client: NewClient(),
		State:  ServerStateStopped,
	}

	for _, ext := range extensions {
		ext = strings.ToLower(ext)
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		if _, exists := m.extMap[ext]; !exists {
			m.extMap[ext] = name
		}
	}

	logger.DebugCF("lsp.manager", "server registered", map[string]any{
		"server":     name,
		"command":    config.Command,
		"extensions": extensions,
	})
	return nil
}

// getServerForFile returns the server entry for the given file path based
// on its extension. Returns nil if no server handles this file type.
func (m *Manager) getServerForFile(filePath string) *serverEntry {
	ext := strings.ToLower(filepath.Ext(filePath))
	if name, ok := m.extMap[ext]; ok {
		return m.servers[name]
	}
	return nil
}

// ensureServerStarted starts the server for the given file path if it is not
// already running. Returns the server entry or an error.
func (m *Manager) ensureServerStarted(filePath string) (*serverEntry, error) {
	entry := m.getServerForFile(filePath)
	if entry == nil {
		return nil, nil
	}

	if entry.State == ServerStateRunning || entry.State == ServerStateStarting {
		return entry, nil
	}

	entry.State = ServerStateStarting
	logger.DebugCF("lsp.manager", "starting server", map[string]any{
		"server":  entry.Name,
		"command": entry.Config.Command,
	})

	if err := entry.Client.Start(entry.Config.Command, entry.Config.Args...); err != nil {
		entry.State = ServerStateError
		return nil, fmt.Errorf("lsp manager: start %q: %w", entry.Name, err)
	}

	// Derive root URI from current working directory.
	cwd, _ := os.Getwd()
	rootURI := "file://" + cwd

	if _, err := entry.Client.Initialize(rootURI); err != nil {
		entry.State = ServerStateError
		return nil, fmt.Errorf("lsp manager: initialize %q: %w", entry.Name, err)
	}

	entry.State = ServerStateRunning
	return entry, nil
}

// SendRequest routes an LSP request to the appropriate server for the given
// file path. Returns the raw JSON result. If no server handles this file type,
// returns nil result and nil error.
func (m *Manager) SendRequest(filePath, method string, params any) (json.RawMessage, error) {
	m.mu.Lock()
	entry, err := m.ensureServerStarted(filePath)
	m.mu.Unlock()

	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, nil
	}

	return entry.Client.SendRequest(method, params)
}

// SendRequestTyped is like SendRequest but unmarshals the result into dst.
// dst must be a pointer to the desired result type.
func (m *Manager) SendRequestTyped(filePath, method string, params any, dst any) error {
	raw, err := m.SendRequest(filePath, method, params)
	if err != nil {
		return err
	}
	if raw == nil {
		return nil
	}
	return json.Unmarshal(raw, dst)
}

// OpenFile sends a textDocument/didOpen notification to the LSP server
// for the given file. The file content is provided so the server can
// index it for code intelligence operations.
func (m *Manager) OpenFile(filePath, content string) error {
	m.mu.Lock()
	entry, err := m.ensureServerStarted(filePath)
	m.mu.Unlock()

	if err != nil {
		return err
	}
	if entry == nil {
		return nil
	}

	absPath, _ := filepath.Abs(filePath)
	fileURI := "file://" + absPath

	// Check if already open on this server.
	if existing, ok := m.openFiles[fileURI]; ok && existing == entry.Name {
		return nil
	}

	ext := strings.ToLower(filepath.Ext(filePath))
	languageID := extToLanguageID(ext)

	err = entry.Client.SendNotification("textDocument/didOpen", map[string]any{
		"textDocument": map[string]any{
			"uri":        fileURI,
			"languageId": languageID,
			"version":    1,
			"text":       content,
		},
	})
	if err != nil {
		return fmt.Errorf("lsp manager: didOpen %q: %w", filePath, err)
	}

	m.openFiles[fileURI] = entry.Name
	return nil
}

// CloseFile sends a textDocument/didClose notification to the LSP server
// for the given file.
func (m *Manager) CloseFile(filePath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry := m.getServerForFile(filePath)
	if entry == nil || entry.State != ServerStateRunning {
		return nil
	}

	absPath, _ := filepath.Abs(filePath)
	fileURI := "file://" + absPath

	if m.openFiles[fileURI] != entry.Name {
		return nil
	}

	err := entry.Client.SendNotification("textDocument/didClose", map[string]any{
		"textDocument": map[string]any{
			"uri": fileURI,
		},
	})
	if err != nil {
		return fmt.Errorf("lsp manager: didClose %q: %w", filePath, err)
	}

	delete(m.openFiles, fileURI)
	return nil
}

// ChangeFile sends a textDocument/didChange notification to the LSP server
// for the given file. The file content is provided so the server can update
// its internal index. If the file has not been opened yet, it is opened first
// (LSP servers require didOpen before didChange).
func (m *Manager) ChangeFile(filePath, content string) error {
	entry := m.getServerForFile(filePath)
	if entry == nil || entry.State != ServerStateRunning {
		return nil
	}

	absPath, _ := filepath.Abs(filePath)
	fileURI := "file://" + absPath

	// If file hasn't been opened yet, open it first.
	if m.openFiles[fileURI] != entry.Name {
		return m.OpenFile(filePath, content)
	}

	if err := entry.Client.SendNotification("textDocument/didChange", map[string]any{
		"textDocument": map[string]any{
			"uri":     fileURI,
			"version": 1,
		},
		"contentChanges": []map[string]any{
			{"text": content},
		},
	}); err != nil {
		return fmt.Errorf("lsp manager: didChange %q: %w", filePath, err)
	}

	return nil
}

// SaveFile sends a textDocument/didSave notification to the LSP server for
// the given file. This triggers the server to re-run diagnostics and other
// save-time analysis. If no server handles this file type, it is a no-op.
func (m *Manager) SaveFile(filePath string) error {
	entry := m.getServerForFile(filePath)
	if entry == nil || entry.State != ServerStateRunning {
		return nil
	}

	absPath, _ := filepath.Abs(filePath)
	fileURI := "file://" + absPath

	if err := entry.Client.SendNotification("textDocument/didSave", map[string]any{
		"textDocument": map[string]any{
			"uri": fileURI,
		},
	}); err != nil {
		return fmt.Errorf("lsp manager: didSave %q: %w", filePath, err)
	}

	return nil
}

// GetAllServers returns a snapshot of all registered server names.
// The returned map keys are server names; values are the corresponding ServerConfigs.
func (m *Manager) GetAllServers() map[string]ServerConfig {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make(map[string]ServerConfig, len(m.servers))
	for name, entry := range m.servers {
		result[name] = entry.Config
	}
	return result
}

// OnNotification registers a handler for incoming notifications from the LSP
// server identified by serverName. The handler is called with the raw JSON
// params of notifications matching the given method (e.g.,
// "textDocument/publishDiagnostics").
//
// The server must already be registered. If it is not running yet, the handler
// is still registered and will become active once the server starts.
func (m *Manager) OnNotification(serverName, method string, handler func(json.RawMessage)) error {
	m.mu.Lock()
	entry, ok := m.servers[serverName]
	m.mu.Unlock()

	if !ok {
		return fmt.Errorf("lsp manager: server %q not registered", serverName)
	}

	// Wrap the handler so it runs with the server name context.
	entry.Client.OnNotification(method, handler)
	return nil
}

// IsFileOpen returns whether the given file has been opened on an LSP server.
func (m *Manager) IsFileOpen(filePath string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	absPath, _ := filepath.Abs(filePath)
	fileURI := "file://" + absPath
	_, ok := m.openFiles[fileURI]
	return ok
}

// Shutdown stops all running LSP servers and clears internal state.
func (m *Manager) Shutdown() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errs []error
	for name, entry := range m.servers {
		if entry.State == ServerStateRunning {
			if err := entry.Client.Shutdown(); err != nil {
				errs = append(errs, fmt.Errorf("shutdown %q: %w", name, err))
			}
		}
		entry.State = ServerStateStopped
	}

	m.servers = make(map[string]*serverEntry)
	m.extMap = make(map[string]string)
	m.openFiles = make(map[string]string)

	if len(errs) > 0 {
		return fmt.Errorf("lsp manager shutdown errors: %v", errs)
	}
	return nil
}

// ─── Global singleton ───────────────────────────────────────────────────────

// InitializationState represents the initialization status of the global LSP
// manager singleton.
type InitializationState int

const (
	// InitStateNotStarted indicates the LSP manager has not been initialized.
	InitStateNotStarted InitializationState = iota
	// InitStatePending indicates initialization is in progress.
	InitStatePending
	// InitStateSuccess indicates the LSP manager initialized successfully.
	InitStateSuccess
	// InitStateFailed indicates the LSP manager failed to initialize.
	InitStateFailed
)

// String returns a human-readable representation of the initialization state.
func (s InitializationState) String() string {
	switch s {
	case InitStateNotStarted:
		return "not-started"
	case InitStatePending:
		return "pending"
	case InitStateSuccess:
		return "success"
	case InitStateFailed:
		return "failed"
	default:
		return "unknown"
	}
}

var (
	globalMu       sync.Mutex
	globalManager  *Manager
	globalState    InitializationState
	globalInitErr  error
)

// GetManager returns the global LSP manager singleton, or nil if it has not
// been initialized or initialization failed.
func GetManager() *Manager {
	globalMu.Lock()
	defer globalMu.Unlock()
	if globalState == InitStateFailed {
		return nil
	}
	return globalManager
}

// SetManager replaces the global LSP manager singleton. This is primarily
// intended for testing.
func SetManager(m *Manager) {
	globalMu.Lock()
	defer globalMu.Unlock()
	globalManager = m
	if m != nil {
		globalState = InitStateSuccess
		globalInitErr = nil
	} else {
		globalState = InitStateNotStarted
		globalInitErr = nil
	}
}

// Initialize creates the global LSP manager singleton. It is idempotent:
// subsequent calls after a successful initialization are no-ops. If a
// previous initialization failed, calling Initialize again will retry.
func Initialize() error {
	globalMu.Lock()
	if globalManager != nil && globalState == InitStateSuccess {
		globalMu.Unlock()
		logger.DebugCF("lsp.manager", "global manager already initialized", nil)
		return nil
	}

	// Reset state for retry if previous init failed.
	if globalState == InitStateFailed {
		globalManager = nil
		globalInitErr = nil
	}

	globalManager = NewManager()
	globalState = InitStatePending
	globalMu.Unlock()

	logger.DebugCF("lsp.manager", "global manager initializing", nil)

	// Synchronous initialization: create manager is sufficient.
	// Server registration and diagnostic handler setup are done by the caller.
	globalMu.Lock()
	globalState = InitStateSuccess
	globalInitErr = nil
	globalMu.Unlock()

	logger.DebugCF("lsp.manager", "global manager initialized successfully", nil)
	return nil
}

// Shutdown stops the global LSP manager and clears the singleton. Errors
// during shutdown are logged but not propagated since this is typically called
// during application exit.
func Shutdown() error {
	globalMu.Lock()
	if globalManager == nil {
		globalMu.Unlock()
		return nil
	}
	mgr := globalManager
	globalManager = nil
	globalState = InitStateNotStarted
	globalInitErr = nil
	globalMu.Unlock()

	if err := mgr.Shutdown(); err != nil {
		logger.DebugCF("lsp.manager", "global manager shutdown error", map[string]any{
			"error": err.Error(),
		})
		return err
	}

	logger.DebugCF("lsp.manager", "global manager shut down successfully", nil)
	return nil
}

// GetInitializationStatus returns the current status of the global LSP manager
// initialization and any error from the last failed attempt.
func GetInitializationStatus() (InitializationState, error) {
	globalMu.Lock()
	defer globalMu.Unlock()
	return globalState, globalInitErr
}

// extToLanguageID maps common file extensions to LSP language IDs.
func extToLanguageID(ext string) string {
	switch ext {
	case ".go":
		return "go"
	case ".ts", ".tsx":
		return "typescript"
	case ".js", ".jsx":
		return "javascript"
	case ".py":
		return "python"
	case ".rs":
		return "rust"
	case ".java":
		return "java"
	case ".c", ".h":
		return "c"
	case ".cpp", ".cc", ".cxx", ".hpp":
		return "cpp"
	case ".cs":
		return "csharp"
	case ".rb":
		return "ruby"
	case ".php":
		return "php"
	case ".swift":
		return "swift"
	case ".kt":
		return "kotlin"
	case ".scala":
		return "scala"
	case ".vue":
		return "vue"
	case ".svelte":
		return "svelte"
	case ".html":
		return "html"
	case ".css":
		return "css"
	case ".scss":
		return "scss"
	case ".json":
		return "json"
	case ".yaml", ".yml":
		return "yaml"
	case ".xml":
		return "xml"
	case ".md":
		return "markdown"
	case ".sql":
		return "sql"
	case ".sh", ".bash":
		return "shellscript"
	case ".lua":
		return "lua"
	case ".r":
		return "r"
	default:
		return "plaintext"
	}
}
