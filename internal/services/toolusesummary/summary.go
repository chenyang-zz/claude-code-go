package toolusesummary

import (
	"context"
	"fmt"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/services/haiku"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// ToolInfo captures one completed tool invocation in a batch.
type ToolInfo struct {
	// Name is the provider-visible tool name, e.g. "Read" or "Bash".
	Name string
	// Input carries the decoded tool arguments. Marshalled with
	// encoding/json before insertion into the prompt.
	Input any
	// Output carries the tool execution result. Marshalled with
	// encoding/json before insertion into the prompt.
	Output any
}

// SummaryParams aggregates the inputs to GenerateToolUseSummary.
type SummaryParams struct {
	// Tools is the ordered batch of completed tool invocations.
	Tools []ToolInfo
	// LastAssistantText is an optional context prefix capped at the first
	// 200 characters before insertion into the prompt template.
	LastAssistantText string
	// IsNonInteractiveSession is forwarded to the haiku layer for
	// log correlation.
	IsNonInteractiveSession bool
}

// Service holds a haiku.Querier used to dispatch summary requests. The
// querier is supplied at construction so tests can inject a stub.
type Service struct {
	querier haiku.Querier
}

// NewService constructs a Service bound to the given Querier. Returns nil
// when querier is nil so callers can detect an unwired runtime.
func NewService(querier haiku.Querier) *Service {
	if querier == nil {
		return nil
	}
	return &Service{querier: querier}
}

// Generate sends a tool use summary request through the bound haiku
// querier. Returns ("", nil) for any of the following non-error paths so the
// caller can treat the summary as best-effort:
//   - feature flag disabled
//   - empty Tools batch
//   - haiku query returns an error (logged)
//   - haiku query returns empty text
//
// All hard errors are absorbed in the haiku layer; this matches the TS
// `catch -> return null` semantics.
func (s *Service) Generate(ctx context.Context, params SummaryParams) (string, error) {
	if s == nil || s.querier == nil {
		return "", nil
	}
	if !IsToolUseSummaryEnabled() {
		return "", nil
	}
	if len(params.Tools) == 0 {
		return "", nil
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}

	toolSummaries := formatToolBatch(params.Tools)

	contextPrefix := ""
	if params.LastAssistantText != "" {
		trimmed := params.LastAssistantText
		if len(trimmed) > lastAssistantTextLimit {
			trimmed = trimmed[:lastAssistantTextLimit]
		}
		contextPrefix = fmt.Sprintf(contextPrefixTemplate, trimmed)
	}

	userPrompt := fmt.Sprintf(userPromptTemplate, contextPrefix, toolSummaries)

	result, err := s.querier.Query(ctx, haiku.QueryParams{
		SystemPrompt:            SystemPrompt,
		UserPrompt:              userPrompt,
		EnablePromptCaching:     true,
		IsNonInteractiveSession: params.IsNonInteractiveSession,
		QuerySource:             querySource,
	})
	if err != nil {
		logger.DebugCF("tool_use_summary", "haiku_query_failed", map[string]any{
			"error": err.Error(),
		})
		return "", nil
	}
	if result == nil {
		return "", nil
	}

	summary := strings.TrimSpace(result.Text)
	if summary == "" {
		return "", nil
	}
	return summary, nil
}

// Generate dispatches the request through the active package-level service.
// Returns ("", nil) when the service is uninitialised.
func Generate(ctx context.Context, params SummaryParams) (string, error) {
	svc := currentService()
	if svc == nil {
		return "", nil
	}
	return svc.Generate(ctx, params)
}
