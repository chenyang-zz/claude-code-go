package haiku

import (
	"context"
	"fmt"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
	"github.com/sheepzhao/claude-code-go/internal/core/model"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// Service holds the model client used to dispatch Haiku queries.
type Service struct {
	client model.Client
}

// NewService constructs a Service bound to the given model client. Returns
// nil when client is nil so callers can detect an unwired runtime.
func NewService(client model.Client) *Service {
	if client == nil {
		return nil
	}
	return &Service{client: client}
}

// Query sends a single Haiku request and aggregates the streaming response
// into a QueryResult.
//
// Returns ErrHaikuDisabled when the feature flag is explicitly disabled, or
// ErrClientUnavailable when the service was constructed without a client.
// Other errors are forwarded from the underlying model client and may be
// classified with IsRateLimit / IsNetwork / IsAPIError.
func (s *Service) Query(ctx context.Context, params QueryParams) (*QueryResult, error) {
	if s == nil || s.client == nil {
		return nil, ErrClientUnavailable
	}
	if !IsHaikuEnabled() {
		return nil, ErrHaikuDisabled
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	modelName := params.Model
	if modelName == "" {
		modelName = DefaultHaikuModel
	}
	maxTokens := params.MaxOutputTokens
	if maxTokens <= 0 {
		maxTokens = DefaultMaxOutputTokens
	}

	req := model.Request{
		Model:               modelName,
		System:              params.SystemPrompt,
		MaxOutputTokens:     maxTokens,
		EnablePromptCaching: params.EnablePromptCaching,
		Messages: []message.Message{
			{
				Role: message.RoleUser,
				Content: []message.ContentPart{
					message.TextPart(params.UserPrompt),
				},
			},
		},
	}

	logger.DebugCF("haiku", "query_start", map[string]any{
		"model":           modelName,
		"prompt_caching":  params.EnablePromptCaching,
		"query_source":    params.QuerySource,
		"non_interactive": params.IsNonInteractiveSession,
	})

	stream, err := s.client.Stream(ctx, req)
	if err != nil {
		logger.DebugCF("haiku", "stream_failed", map[string]any{
			"error":        err.Error(),
			"query_source": params.QuerySource,
		})
		return nil, fmt.Errorf("haiku: stream failed: %w", err)
	}

	var (
		builder    strings.Builder
		stopReason model.StopReason
		usage      Usage
	)

	for event := range stream {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		switch event.Type {
		case model.EventTypeTextDelta:
			builder.WriteString(event.Text)
		case model.EventTypeError:
			logger.DebugCF("haiku", "stream_error", map[string]any{
				"error":        event.Error,
				"query_source": params.QuerySource,
			})
			return nil, fmt.Errorf("haiku: model error: %s", event.Error)
		case model.EventTypeDone:
			if event.StopReason != "" {
				stopReason = event.StopReason
			}
			if event.Usage != nil {
				usage = Usage{
					InputTokens:              event.Usage.InputTokens,
					OutputTokens:             event.Usage.OutputTokens,
					CacheCreationInputTokens: event.Usage.CacheCreationInputTokens,
					CacheReadInputTokens:     event.Usage.CacheReadInputTokens,
				}
			}
		}
	}

	result := &QueryResult{
		Text:       strings.TrimSpace(builder.String()),
		StopReason: string(stopReason),
		Usage:      usage,
	}

	logger.DebugCF("haiku", "query_done", map[string]any{
		"text_len":      len(result.Text),
		"input_tokens":  usage.InputTokens,
		"output_tokens": usage.OutputTokens,
		"query_source":  params.QuerySource,
	})

	return result, nil
}
