package registry

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/platform/mcp/client"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

var lastRegistry *ServerRegistry

// SetLastRegistry stores the most recently connected registry for observability.
func SetLastRegistry(r *ServerRegistry) {
	lastRegistry = r
}

// GetLastRegistry returns the most recently connected registry, or nil.
func GetLastRegistry() *ServerRegistry {
	return lastRegistry
}

// ServerStatus describes the runtime state of a single MCP server.
type ServerStatus string

const (
	StatusConnected ServerStatus = "connected"
	StatusFailed    ServerStatus = "failed"
	StatusDisabled  ServerStatus = "disabled"
)

// Entry holds one server configuration together with its runtime connection.
type Entry struct {
	Name   string
	Config client.ServerConfig
	Status ServerStatus
	Client *client.Client
	// Capabilities stores the initialize-time capability snapshot returned by the server.
	Capabilities client.ServerCapabilities
	// Tools stores the latest snapshot returned by tools/list.
	Tools []client.Tool
	// Resources stores the latest snapshot returned by resources/list.
	Resources []client.Resource
	// Prompts stores the latest snapshot returned by prompts/list.
	Prompts []client.Prompt
	// Instructions stores the server-provided usage guidance from the initialize handshake.
	Instructions string
	Error        string
}

const snapshotRefreshTimeout = 5 * time.Second

// ServerRegistry manages the lifecycle of configured MCP servers.
type ServerRegistry struct {
	entries []Entry
	mu      sync.RWMutex
}

// NewServerRegistry creates an empty registry.
func NewServerRegistry() *ServerRegistry {
	return &ServerRegistry{}
}

// LoadConfigs populates the registry from a raw settings map.
// Each key is the server name; the value is parsed as a ServerConfig.
func (r *ServerRegistry) LoadConfigs(configs map[string]client.ServerConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for name, cfg := range configs {
		r.entries = append(r.entries, Entry{
			Name:   name,
			Config: cfg,
			Status: StatusDisabled,
		})
	}
}

// ConnectAll attempts to start every configured server.
func (r *ServerRegistry) ConnectAll(ctx context.Context) {
	r.mu.Lock()
	entries := make([]Entry, len(r.entries))
	copy(entries, r.entries)
	r.mu.Unlock()

	var wg sync.WaitGroup
	for i := range entries {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			r.connectOne(ctx, idx)
		}(i)
	}
	wg.Wait()
}

// connectOne starts a single server and updates its entry atomically.
func (r *ServerRegistry) connectOne(ctx context.Context, idx int) {
	r.mu.RLock()
	entry := r.entries[idx]
	r.mu.RUnlock()

	transport, err := newTransportForEntry(ctx, entry)
	if err != nil {
		r.updateStatus(idx, StatusFailed, fmt.Sprintf("transport: %v", err))
		return
	}

	c := client.NewClient(transport)
	result, err := c.Initialize(ctx, client.InitializeRequest{
		ProtocolVersion: "2024-11-05",
		Capabilities: client.ClientCapabilities{
			Roots: map[string]any{},
		},
		ClientInfo: client.Implementation{
			Name:    "claude-code-go",
			Version: "0.1.0",
		},
	})
	if err != nil {
		_ = c.Close()
		r.updateStatus(idx, StatusFailed, fmt.Sprintf("initialize: %v", err))
		return
	}

	logger.DebugCF("mcp", "server connected", map[string]any{
		"server":           entry.Name,
		"protocol_version": result.ProtocolVersion,
		"server_name":      result.ServerInfo.Name,
	})

	r.mu.Lock()
	r.entries[idx].Client = c
	r.entries[idx].Status = StatusConnected
	r.entries[idx].Capabilities = result.Capabilities
	r.entries[idx].Instructions = result.Instructions
	r.entries[idx].Error = ""
	r.mu.Unlock()

	r.registerNotificationHandlers(idx)
	r.refreshConnectedSnapshots(ctx, idx)
}

// SetInstructions records the latest server-provided usage guidance for one entry.
func (r *ServerRegistry) SetInstructions(name string, instructions string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.entries {
		if r.entries[i].Name == name {
			r.entries[i].Instructions = instructions
			return
		}
	}
}

// registerNotificationHandlers wires list_changed notifications to snapshot refreshes.
func (r *ServerRegistry) registerNotificationHandlers(idx int) {
	r.mu.RLock()
	if idx >= len(r.entries) {
		r.mu.RUnlock()
		return
	}
	entry := r.entries[idx]
	r.mu.RUnlock()

	if entry.Client == nil {
		return
	}

	if entry.Capabilities.Tools != nil && entry.Capabilities.Tools.ListChanged {
		_ = entry.Client.SetNotificationHandler("tools/list_changed", func(client.JSONRPCNotification) {
			r.refreshToolsSnapshot(context.Background(), idx)
		})
	}
	if entry.Capabilities.Resources != nil && entry.Capabilities.Resources.ListChanged {
		_ = entry.Client.SetNotificationHandler("resources/list_changed", func(client.JSONRPCNotification) {
			r.refreshResourcesSnapshot(context.Background(), idx)
		})
	}
	if entry.Capabilities.Prompts != nil && entry.Capabilities.Prompts.ListChanged {
		_ = entry.Client.SetNotificationHandler("prompts/list_changed", func(client.JSONRPCNotification) {
			r.refreshPromptsSnapshot(context.Background(), idx)
		})
	}
}

// refreshConnectedSnapshots populates all supported snapshots immediately after connect.
func (r *ServerRegistry) refreshConnectedSnapshots(ctx context.Context, idx int) {
	r.refreshToolsSnapshot(ctx, idx)
	r.refreshResourcesSnapshot(ctx, idx)
	r.refreshPromptsSnapshot(ctx, idx)
}

// newTransportForEntry creates the appropriate MCP transport for one server config.
// stdio remains the default for backwards compatibility; sse/ws now route through
// the new remote transport bridge.
func newTransportForEntry(ctx context.Context, entry Entry) (client.Transport, error) {
	switch entry.Config.Type {
	case "", "stdio":
		return client.NewStdioClientTransport(
			entry.Config.Command,
			entry.Config.Args,
			entry.Config.Env,
		)
	case "sse":
		return client.NewSSEClientTransport(ctx, entry.Config.URL, entry.Config.Headers)
	case "ws":
		return client.NewWebSocketClientTransport(ctx, entry.Config.URL, entry.Config.Headers)
	case "http":
		return client.NewHTTPClientTransport(ctx, entry.Config.URL, entry.Config.Headers)
	case "claudeai-proxy", "sdk":
		return nil, fmt.Errorf("unsupported transport type %q", entry.Config.Type)
	default:
		return nil, fmt.Errorf("unsupported transport type %q", entry.Config.Type)
	}
}

// refreshToolsSnapshot refreshes the tools snapshot for one connected entry.
func (r *ServerRegistry) refreshToolsSnapshot(ctx context.Context, idx int) {
	entry, ok := r.connectedEntry(idx)
	if !ok || entry.Client == nil {
		return
	}

	refreshCtx, cancel := context.WithTimeout(ctx, snapshotRefreshTimeout)
	defer cancel()

	result, err := entry.Client.ListTools(refreshCtx)
	if err != nil {
		logger.WarnCF("mcp", "refresh tools snapshot failed", map[string]any{
			"server": entry.Name,
			"error":  err.Error(),
		})
		return
	}

	r.mu.Lock()
	if idx < len(r.entries) && r.entries[idx].Name == entry.Name {
		r.entries[idx].Tools = append([]client.Tool(nil), result.Tools...)
	}
	r.mu.Unlock()
}

// refreshResourcesSnapshot refreshes the resources snapshot for one connected entry.
func (r *ServerRegistry) refreshResourcesSnapshot(ctx context.Context, idx int) {
	entry, ok := r.connectedEntry(idx)
	if !ok || entry.Client == nil {
		return
	}

	refreshCtx, cancel := context.WithTimeout(ctx, snapshotRefreshTimeout)
	defer cancel()

	result, err := entry.Client.ListResources(refreshCtx)
	if err != nil {
		logger.WarnCF("mcp", "refresh resources snapshot failed", map[string]any{
			"server": entry.Name,
			"error":  err.Error(),
		})
		return
	}

	r.mu.Lock()
	if idx < len(r.entries) && r.entries[idx].Name == entry.Name {
		r.entries[idx].Resources = append([]client.Resource(nil), result.Resources...)
	}
	r.mu.Unlock()
}

// refreshPromptsSnapshot refreshes the prompts snapshot for one connected entry.
func (r *ServerRegistry) refreshPromptsSnapshot(ctx context.Context, idx int) {
	entry, ok := r.connectedEntry(idx)
	if !ok || entry.Client == nil {
		return
	}

	refreshCtx, cancel := context.WithTimeout(ctx, snapshotRefreshTimeout)
	defer cancel()

	result, err := entry.Client.ListPrompts(refreshCtx)
	if err != nil {
		logger.WarnCF("mcp", "refresh prompts snapshot failed", map[string]any{
			"server": entry.Name,
			"error":  err.Error(),
		})
		return
	}

	r.mu.Lock()
	if idx < len(r.entries) && r.entries[idx].Name == entry.Name {
		r.entries[idx].Prompts = append([]client.Prompt(nil), result.Prompts...)
	}
	r.mu.Unlock()
}

// connectedEntry returns a snapshot of one connected entry.
func (r *ServerRegistry) connectedEntry(idx int) (Entry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if idx < 0 || idx >= len(r.entries) {
		return Entry{}, false
	}
	entry := r.entries[idx]
	if entry.Status != StatusConnected {
		return Entry{}, false
	}
	return entry, true
}

// updateStatus updates the status and error message for a single entry.
func (r *ServerRegistry) updateStatus(idx int, status ServerStatus, errMsg string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if idx < len(r.entries) {
		r.entries[idx].Status = status
		r.entries[idx].Error = errMsg
	}
}

// CloseAll shuts down every active connection.
func (r *ServerRegistry) CloseAll() {
	r.mu.Lock()
	entries := make([]Entry, len(r.entries))
	copy(entries, r.entries)
	r.mu.Unlock()

	var wg sync.WaitGroup
	for _, e := range entries {
		if e.Client == nil {
			continue
		}
		wg.Add(1)
		go func(c *client.Client) {
			defer wg.Done()
			_ = c.Close()
		}(e.Client)
	}
	wg.Wait()
}

// Connected returns only the successfully connected entries.
func (r *ServerRegistry) Connected() []Entry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var out []Entry
	for _, e := range r.entries {
		if e.Status == StatusConnected {
			out = append(out, e)
		}
	}
	return out
}

// List returns a snapshot of all entries.
func (r *ServerRegistry) List() []Entry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]Entry, len(r.entries))
	copy(out, r.entries)
	return out
}
