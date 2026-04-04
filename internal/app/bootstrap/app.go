package bootstrap

import (
	"context"
	"fmt"

	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
	"github.com/sheepzhao/claude-code-go/internal/platform/api/anthropic"
	platformconfig "github.com/sheepzhao/claude-code-go/internal/platform/config"
	"github.com/sheepzhao/claude-code-go/internal/runtime/engine"
	"github.com/sheepzhao/claude-code-go/internal/runtime/repl"
	"github.com/sheepzhao/claude-code-go/internal/ui/console"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// EngineFactory constructs the engine selected by the resolved runtime config.
type EngineFactory func(cfg coreconfig.Config) (engine.Engine, error)

// App wires together the minimum batch-07 runtime needed by cmd/cc.
type App struct {
	// Config stores the resolved runtime configuration for observability and tests.
	Config coreconfig.Config
	// Runner owns the one-turn REPL execution flow.
	Runner *repl.Runner
}

// NewApp builds the production app wiring from the default config loader.
func NewApp() (*App, error) {
	loader, err := platformconfig.NewDefaultFileLoader()
	if err != nil {
		return nil, err
	}

	return NewAppWithDependencies(loader, DefaultEngineFactory)
}

// NewAppWithDependencies builds the app from explicit loader and engine dependencies.
func NewAppWithDependencies(loader coreconfig.Loader, engineFactory EngineFactory) (*App, error) {
	cfg, err := loader.Load(context.Background())
	if err != nil {
		return nil, err
	}

	eng, err := engineFactory(cfg)
	if err != nil {
		return nil, err
	}

	renderer := console.NewStreamRenderer(console.NewPrinter(nil))

	logger.DebugCF("bootstrap", "constructed application", map[string]any{
		"provider": cfg.Provider,
		"model":    cfg.Model,
	})

	return &App{
		Config: cfg,
		Runner: repl.NewRunner(eng, renderer),
	}, nil
}

// DefaultEngineFactory selects the minimum provider implementation supported by batch-07.
func DefaultEngineFactory(cfg coreconfig.Config) (engine.Engine, error) {
	switch cfg.Provider {
	case "", "anthropic":
		client := anthropic.NewClient(anthropic.Config{
			APIKey:     cfg.APIKey,
			BaseURL:    cfg.APIBaseURL,
			HTTPClient: nil,
		})
		return engine.New(client, cfg.Model), nil
	default:
		return nil, fmt.Errorf("unsupported provider %q", cfg.Provider)
	}
}

// Run forwards execution to the configured runner.
func (a *App) Run(ctx context.Context, args []string) error {
	return a.Runner.Run(ctx, args)
}
