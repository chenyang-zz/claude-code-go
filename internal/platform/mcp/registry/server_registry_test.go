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
							printf '%s\n' '{"jsonrpc":"2.0","id":"'"$id"'","result":{"tools":[{"name":"tool_one","description":"Tool one"}]}}'
							;;
						resources/list)
							printf '%s\n' '{"jsonrpc":"2.0","id":"'"$id"'","result":{"resources":[{"uri":"file:///tmp/a","name":"config","description":"Config file"}]}}'
							;;
						prompts/list)
							printf '%s\n' '{"jsonrpc":"2.0","id":"'"$id"'","result":{"prompts":[{"name":"summarize","description":"Summarize"}]}}'
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
	if len(entries[0].Tools) != 1 || entries[0].Tools[0].Name != "tool_one" {
		t.Fatalf("tools = %#v", entries[0].Tools)
	}
	if len(entries[0].Resources) != 1 || entries[0].Resources[0].URI != "file:///tmp/a" {
		t.Fatalf("resources = %#v", entries[0].Resources)
	}
	if len(entries[0].Prompts) != 1 || entries[0].Prompts[0].Name != "summarize" {
		t.Fatalf("prompts = %#v", entries[0].Prompts)
	}
}

func TestServerRegistryRefreshesToolsSnapshotOnNotification(t *testing.T) {
	r := NewServerRegistry()
	r.LoadConfigs(map[string]client.ServerConfig{
		"caps": {
			Command: "sh",
			Args: []string{"-c", `
				tools_count=0
				while IFS= read -r line; do
					id=$(printf '%s' "$line" | sed -n 's/.*"id":"\([^"]*\)".*/\1/p')
					method=$(printf '%s' "$line" | sed -n 's/.*"method":"\([^"]*\)".*/\1/p')
					case "$method" in
						initialize)
							printf '%s\n' '{"jsonrpc":"2.0","id":"'"$id"'","result":{"protocolVersion":"2024-11-05","capabilities":{"tools":{"listChanged":true},"resources":{},"prompts":{}},"serverInfo":{"name":"caps","version":"1.0"}}}'
							(sleep 0.2; printf '%s\n' '{"jsonrpc":"2.0","method":"tools/list_changed"}') &
							;;
						tools/list)
							tools_count=$((tools_count + 1))
							if [ "$tools_count" -eq 1 ]; then
								printf '%s\n' '{"jsonrpc":"2.0","id":"'"$id"'","result":{"tools":[{"name":"tool_one"}]}}'
							else
								printf '%s\n' '{"jsonrpc":"2.0","id":"'"$id"'","result":{"tools":[{"name":"tool_two"}]}}'
							fi
							;;
						resources/list)
							printf '%s\n' '{"jsonrpc":"2.0","id":"'"$id"'","result":{"resources":[{"uri":"file:///tmp/a","name":"config"}]}}'
							;;
						prompts/list)
							printf '%s\n' '{"jsonrpc":"2.0","id":"'"$id"'","result":{"prompts":[{"name":"summarize"}]}}'
							;;
					esac
				done
			`},
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	r.ConnectAll(ctx)

	waitForEntryTools := func(want string) bool {
		for _, entry := range r.List() {
			if entry.Name != "caps" {
				continue
			}
			return len(entry.Tools) == 1 && entry.Tools[0].Name == want
		}
		return false
	}

	deadline := time.After(5 * time.Second)
	for {
		if waitForEntryTools("tool_two") {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for refreshed tools snapshot: %#v", r.List())
		default:
			time.Sleep(50 * time.Millisecond)
		}
	}
}

func TestSetGetLastRegistry(t *testing.T) {
	r := NewServerRegistry()
	SetLastRegistry(r)
	if GetLastRegistry() != r {
		t.Fatal("GetLastRegistry did not return the set registry")
	}
}
