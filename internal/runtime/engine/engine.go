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
	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
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
	// ToolCatalog stores the provider-facing tool declarations attached to each request by default.
	ToolCatalog []model.ToolDefinition
}

// New builds the minimum single-turn engine.
func New(client model.Client, defaultModel string, tools ...model.ToolDefinition) *Runtime {
	return &Runtime{
		Client:       client,
		DefaultModel: defaultModel,
		ToolCatalog:  append([]model.ToolDefinition(nil), tools...),
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
		Tools:    e.ToolCatalog,
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
			case model.EventTypeToolUse:
				if item.ToolUse == nil {
					out <- event.Event{
						Type:      event.TypeError,
						Timestamp: time.Now(),
						Payload: event.ErrorPayload{
							Message: "tool use event missing payload",
						},
					}
					continue
				}
				out <- event.Event{
					Type:      event.TypeToolCallStarted,
					Timestamp: time.Now(),
					Payload: event.ToolCallPayload{
						ID:    item.ToolUse.ID,
						Name:  item.ToolUse.Name,
						Input: item.ToolUse.Input,
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

// DescribeTools converts a tool registry into provider-facing tool definitions.
func DescribeTools(registry coretool.Registry) []model.ToolDefinition {
	if registry == nil {
		return nil
	}

	registered := registry.List()
	descriptions := make([]model.ToolDefinition, 0, len(registered))
	for _, item := range registered {
		if item == nil {
			continue
		}
		descriptions = append(descriptions, model.ToolDefinition{
			Name:        item.Name(),
			Description: item.Description(),
			InputSchema: item.InputSchema().JSONSchema(),
		})
	}
	return descriptions
}
