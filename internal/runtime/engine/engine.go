package engine

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/conversation"
	"github.com/sheepzhao/claude-code-go/internal/core/event"
	"github.com/sheepzhao/claude-code-go/internal/core/message"
	"github.com/sheepzhao/claude-code-go/internal/core/model"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// Engine executes one conversation request and produces runtime events.
type Engine interface {
	Run(ctx context.Context, req conversation.RunRequest) (event.Stream, error)
}

// Runtime is the minimum single-turn text engine used by batch-07.
type Runtime struct {
	// Client sends the provider request and returns a model event stream.
	Client model.Client
	// DefaultModel is used when the caller does not override the model.
	DefaultModel string
}

// New builds the minimum single-turn engine.
func New(client model.Client, defaultModel string) *Runtime {
	return &Runtime{
		Client:       client,
		DefaultModel: defaultModel,
	}
}

// Run converts a single input turn into a provider stream and maps it back into runtime events.
func (e *Runtime) Run(ctx context.Context, req conversation.RunRequest) (event.Stream, error) {
	messages := req.Messages
	if len(messages) == 0 {
		input := strings.TrimSpace(req.Input)
		if input == "" {
			return nil, fmt.Errorf("missing user input")
		}

		messages = []message.Message{
			{
				Role: message.RoleUser,
				Content: []message.ContentPart{
					{
						Type: "text",
						Text: input,
					},
				},
			},
		}
	}

	logger.DebugCF("engine", "starting single-turn run", map[string]any{
		"session_id":    req.SessionID,
		"message_count": len(messages),
		"model":         e.DefaultModel,
	})

	modelStream, err := e.Client.Stream(ctx, model.Request{
		Model:    e.DefaultModel,
		Messages: messages,
	})
	if err != nil {
		return nil, err
	}

	out := make(chan event.Event)
	go func() {
		defer close(out)
		for item := range modelStream {
			switch item.Type {
			case model.EventTypeTextDelta:
				out <- event.Event{
					Type:      event.TypeMessageDelta,
					Timestamp: time.Now(),
					Payload: event.MessageDeltaPayload{
						Text: item.Text,
					},
				}
			case model.EventTypeError:
				out <- event.Event{
					Type:      event.TypeError,
					Timestamp: time.Now(),
					Payload: event.ErrorPayload{
						Message: item.Error,
					},
				}
			}
		}

		logger.DebugCF("engine", "single-turn run finished", map[string]any{
			"session_id": req.SessionID,
		})
	}()

	return out, nil
}
