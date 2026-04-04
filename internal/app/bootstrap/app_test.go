package bootstrap

import (
	"context"
	"testing"

	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
	"github.com/sheepzhao/claude-code-go/internal/core/conversation"
	"github.com/sheepzhao/claude-code-go/internal/core/event"
	"github.com/sheepzhao/claude-code-go/internal/runtime/engine"
)

type stubLoader struct {
	cfg coreconfig.Config
}

func (l stubLoader) Load(ctx context.Context) (coreconfig.Config, error) {
	return l.cfg, nil
}

type stubEngine struct{}

func (e stubEngine) Run(ctx context.Context, req conversation.RunRequest) (event.Stream, error) {
	ch := make(chan event.Event)
	close(ch)
	return ch, nil
}

var _ engine.Engine = stubEngine{}

// TestNewAppWithDependenciesLoadsConfig verifies bootstrap wires the runner from resolved config and selected engine.
func TestNewAppWithDependenciesLoadsConfig(t *testing.T) {
	loader := stubLoader{
		cfg: coreconfig.Config{
			Provider: "anthropic",
			Model:    "claude-sonnet-4-5",
			APIKey:   "test-key",
		},
	}

	called := false
	app, err := NewAppWithDependencies(loader, func(cfg coreconfig.Config) (engine.Engine, error) {
		called = true
		if cfg.APIKey != "test-key" {
			t.Fatalf("engine factory cfg = %#v, want api key", cfg)
		}
		return stubEngine{}, nil
	})
	if err != nil {
		t.Fatalf("NewAppWithDependencies() error = %v", err)
	}

	if !called {
		t.Fatal("engine factory was not called")
	}
	if app.Config.Provider != "anthropic" || app.Runner == nil {
		t.Fatalf("NewAppWithDependencies() = %#v, want anthropic config and runner", app)
	}
}
