package bootstrap

import (
	"context"

	"github.com/sheepzhao/claude-code-go/internal/runtime/repl"
)

type App struct {
	Runner *repl.Runner
}

func NewApp() (*App, error) {
	return &App{
		Runner: repl.NewRunner(),
	}, nil
}

func (a *App) Run(ctx context.Context, args []string) error {
	return a.Runner.Run(ctx, args)
}
