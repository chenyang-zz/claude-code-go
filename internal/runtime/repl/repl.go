package repl

import (
	"context"
	"fmt"

	"github.com/sheepzhao/claude-code-go/internal/core/conversation"
	"github.com/sheepzhao/claude-code-go/internal/core/event"
	"github.com/sheepzhao/claude-code-go/internal/runtime/engine"
)

type Runner struct {
	engine engine.Engine
}

func NewRunner() *Runner {
	return &Runner{engine: engine.NewStub()}
}

func (r *Runner) Run(ctx context.Context, args []string) error {
	stream, err := r.engine.Run(ctx, conversation.RunRequest{
		SessionID: "bootstrap",
		Input:     "init",
	})
	if err != nil {
		return err
	}

	for evt := range stream {
		if evt.Type == event.TypeMessageDelta {
			payload, _ := evt.Payload.(event.MessageDeltaPayload)
			fmt.Println(payload.Text)
		}
	}
	return nil
}
