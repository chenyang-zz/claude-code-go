package main

import (
	"context"
	"fmt"
	"os"

	"github.com/sheepzhao/claude-code-go/internal/app/bootstrap"
)

func main() {
	app, runArgs, err := bootstrap.NewAppFromArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "bootstrap error:", err)
		os.Exit(1)
	}

	if err := app.Run(context.Background(), runArgs); err != nil {
		fmt.Fprintln(os.Stderr, "run error:", err)
		os.Exit(1)
	}
}
