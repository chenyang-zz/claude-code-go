package bootstrap

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/tool"
	mcpclient "github.com/sheepzhao/claude-code-go/internal/platform/mcp/client"
	mcpregistry "github.com/sheepzhao/claude-code-go/internal/platform/mcp/registry"
)

// TestRegisterMCPAuthTools registers pseudo auth tools for needs-auth servers.
func TestRegisterMCPAuthTools(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	reg := mcpregistry.NewServerRegistry()
	reg.LoadConfigs(map[string]mcpclient.ServerConfig{
		"needs-auth-srv": {
			Type: "http",
			URL:  srv.URL,
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	reg.ConnectAll(ctx)

	if got := reg.List(); len(got) == 0 || got[0].Status != mcpregistry.StatusNeedsAuth {
		t.Fatalf("registry status = %#v, want needs-auth entry", got)
	}

	registry := tool.NewMemoryRegistry()
	registerMCPAuthTools(registry, reg)

	item, ok := registry.Get("needs-auth-srv__authenticate")
	if !ok {
		t.Fatal("expected needs-auth auth tool to be registered")
	}
	if item == nil {
		t.Fatal("registered auth tool = nil")
	}
}
