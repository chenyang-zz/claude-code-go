package query

import (
	"context"
	"encoding/json"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/conversation"
	"github.com/sheepzhao/claude-code-go/internal/core/event"
	"github.com/sheepzhao/claude-code-go/internal/core/message"
	"github.com/sheepzhao/claude-code-go/internal/runtime/engine"
)

// QueryResult carries the output of a single query execution.
type QueryResult struct {
	// Messages is the full conversation history after this query turn.
	Messages []message.Message
	// Usage contains token usage information.
	Usage string
	// StopReason indicates why the conversation stopped.
	StopReason string
	// Duration is the wall-clock execution time.
	Duration time.Duration
	// Err is non-nil when the query encountered a runtime error.
	Err error
}

// RunQuery executes one query turn using the provided engine Runtime.
// It is a thin wrapper around engine.Runtime.Run() for programmatic use.
func RunQuery(
	ctx context.Context,
	runtime *engine.Runtime,
	req conversation.RunRequest,
) *QueryResult {
	start := time.Now()

	stream, err := runtime.Run(ctx, req)
	if err != nil {
		return &QueryResult{
			Duration: time.Since(start),
			Err:      err,
		}
	}

	var messages []message.Message
	var usageJSON string
	var stopReason string

	for evt := range stream {
		switch evt.Type {
		case event.TypeConversationDone:
			if p, ok := evt.Payload.(event.ConversationDonePayload); ok {
				messages = p.History.Messages
				stopReason = p.StopReason
				if b, err := json.Marshal(p.Usage); err == nil {
					usageJSON = string(b)
				}
			}
		case event.TypeError:
			if p, ok := evt.Payload.(event.ErrorPayload); ok {
				return &QueryResult{
					Messages: messages,
					Duration: time.Since(start),
					Err:      &QueryError{Message: p.Message},
				}
			}
		}
	}

	return &QueryResult{
		Messages:   messages,
		Usage:      usageJSON,
		StopReason: stopReason,
		Duration:   time.Since(start),
	}
}

// QueryError represents an error returned by the query engine.
type QueryError struct {
	Message string
}

func (e *QueryError) Error() string {
	return e.Message
}
