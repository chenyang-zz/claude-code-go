package engine

import (
	"context"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/conversation"
	"github.com/sheepzhao/claude-code-go/internal/core/event"
)

type Engine interface {
	Run(ctx context.Context, req conversation.RunRequest) (event.Stream, error)
}

type StubEngine struct{}

func NewStub() *StubEngine {
	return &StubEngine{}
}

func (e *StubEngine) Run(ctx context.Context, req conversation.RunRequest) (event.Stream, error) {
	ch := make(chan event.Event, 2)
	go func() {
		defer close(ch)
		ch <- event.Event{
			Type:      event.TypeMessageDelta,
			Timestamp: time.Now(),
			Payload: event.MessageDeltaPayload{
				Text: "engine skeleton ready",
			},
		}
	}()
	return ch, nil
}
