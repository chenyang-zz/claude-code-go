package main

import (
	"context"
	"fmt"
	"os"

	"github.com/sheepzhao/claude-code-go/internal/app/bootstrap"
)

func main() {
	app, err := bootstrap.NewApp()
	if err != nil {
		fmt.Fprintln(os.Stderr, "bootstrap error:", err)
		os.Exit(1)
	}

	if err := app.Run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "run error:", err)
		os.Exit(1)
	}
}
