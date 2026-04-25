package registry

import (
	"context"
	"testing"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/platform/mcp/client"
)

func TestNewServerRegistry(t *testing.T) {
	r := NewServerRegistry()
	if r == nil {
		t.Fatal("NewServerRegistry returned nil")
	}
	if len(r.List()) != 0 {
		t.Fatal("new registry should be empty")
	}
}

func TestServerRegistryLoadConfigs(t *testing.T) {
	r := NewServerRegistry()
	r.LoadConfigs(map[string]client.ServerConfig{
		"fs": {Command: "npx", Args: []string{"-y", "@modelcontextprotocol/server-filesystem"}},
	})
	entries := r.List()
	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}
	if entries[0].Name != "fs" {
		t.Fatalf("name = %q", entries[0].Name)
	}
	if entries[0].Status != StatusDisabled {
		t.Fatalf("status = %q, want disabled", entries[0].Status)
	}
}

func TestServerRegistryConnectAllUnsupportedType(t *testing.T) {
	r := NewServerRegistry()
	r.LoadConfigs(map[string]client.ServerConfig{
		"sse": {Type: "sse", Command: "noop"},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	r.ConnectAll(ctx)

	entries := r.List()
	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}
	if entries[0].Status != StatusFailed {
		t.Fatalf("status = %q, want failed", entries[0].Status)
	}
}

func TestServerRegistryCloseAll(t *testing.T) {
	r := NewServerRegistry()
	// CloseAll on empty registry should not panic.
	r.CloseAll()
}

func TestServerRegistryConnected(t *testing.T) {
	r := NewServerRegistry()
	r.LoadConfigs(map[string]client.ServerConfig{
		"a": {Command: "echo"},
		"b": {Command: "echo"},
	})
	// Manually set one entry to connected for the filter test.
	for i := range r.entries {
		if r.entries[i].Name == "a" {
			r.entries[i].Status = StatusConnected
		}
	}

	connected := r.Connected()
	if len(connected) != 1 {
		t.Fatalf("len(connected) = %d, want 1", len(connected))
	}
	if connected[0].Name != "a" {
		t.Fatalf("connected[0].name = %q", connected[0].Name)
	}
}

func TestServerRegistryStoresCapabilities(t *testing.T) {
	r := NewServerRegistry()
	r.LoadConfigs(map[string]client.ServerConfig{
		"caps": {
			Command: "sh",
			Args: []string{"-c", `
				while IFS= read -r line; do
					id=$(printf '%s' "$line" | sed -n 's/.*"id":"\([^"]*\)".*/\1/p')
					method=$(printf '%s' "$line" | sed -n 's/.*"method":"\([^"]*\)".*/\1/p')
					case "$method" in
						initialize)
							printf '%s\n' '{"jsonrpc":"2.0","id":"'"$id"'","result":{"protocolVersion":"2024-11-05","capabilities":{"tools":{},"resources":{},"prompts":{}},"serverInfo":{"name":"caps","version":"1.0"}}}'
							;;
						tools/list)
							printf '%s\n' '{"jsonrpc":"2.0","id":"'"$id"'","result":{"tools":[]}}'
							;;
						resources/list)
							printf '%s\n' '{"jsonrpc":"2.0","id":"'"$id"'","result":{"resources":[]}}'
							;;
						prompts/list)
							printf '%s\n' '{"jsonrpc":"2.0","id":"'"$id"'","result":{"prompts":[]}}'
							;;
					esac
				done
			`},
		},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	r.ConnectAll(ctx)

	entries := r.List()
	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}
	if entries[0].Capabilities.Tools == nil || entries[0].Capabilities.Resources == nil || entries[0].Capabilities.Prompts == nil {
		t.Fatalf("capabilities = %#v", entries[0].Capabilities)
	}
}

func TestSetGetLastRegistry(t *testing.T) {
	r := NewServerRegistry()
	SetLastRegistry(r)
	if GetLastRegistry() != r {
		t.Fatal("GetLastRegistry did not return the set registry")
	}
}
