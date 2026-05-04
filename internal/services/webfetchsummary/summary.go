// Package webfetchsummary provides Haiku-based content summarisation for
// pages fetched by WebFetchTool. The package mirrors the secondary-model
// summarization path in src/tools/WebFetchTool/utils.ts
// (applyPromptToMarkdown) and the prompt template in prompt.ts
// (makeSecondaryModelPrompt).
package webfetchsummary

import (
	"context"
	"errors"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/services/haiku"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// MaxContentLength mirrors MAX_MARKDOWN_LENGTH in utils.ts (100 000 chars).
// Content beyond this limit is truncated before being sent to Haiku to
// avoid "Prompt is too long" errors.
const MaxContentLength = 100_000

// ErrHaikuCallFailed is returned when the Haiku query itself fails and the
// caller should fall back to a best-effort default.
var ErrHaikuCallFailed = errors.New("webfetchsummary: haiku query failed")

// Service holds a haiku.Querier used to dispatch summarization requests.
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

// Summarize calls Haiku to summarize fetched page content based on the
// user's prompt. Returns the assistant text or a fallback message.
//
// Returns ("", nil) when the service is nil, the feature flag is disabled,
// the content is empty, or ctx is cancelled (returns ctx.Err()).
func (s *Service) Summarize(ctx context.Context, content string, prompt string, isPreapprovedDomain bool) (string, error) {
	if s == nil || s.querier == nil {
		return "", nil
	}
	if !IsWebFetchSummaryEnabled() {
		return "", nil
	}
	if strings.TrimSpace(content) == "" {
		return "", nil
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}

	// Truncate content to avoid "Prompt is too long" errors from the
	// secondary model.
	// Truncate to stay within MaxContentLength including the truncation marker.
	var truncated string
	if len(content) > MaxContentLength {
		const marker = "\n\n[Content truncated due to length...]"
		limit := MaxContentLength - len(marker)
		if limit < 0 {
			limit = 0
		}
		truncated = content[:limit] + marker
	} else {
		truncated = content
	}

	modelPrompt := makeSecondaryModelPrompt(truncated, prompt, isPreapprovedDomain)

	logger.DebugCF("webfetchsummary", "summarize_start", map[string]any{
		"content_len":        len(truncated),
		"prompt_len":         len(prompt),
		"preapproved_domain": isPreapprovedDomain,
	})

	result, err := s.querier.Query(ctx, haiku.QueryParams{
		SystemPrompt: "", // empty system prompt, matching TS
		UserPrompt:   modelPrompt,
		QuerySource:  querySource,
	})
	if err != nil {
		logger.DebugCF("webfetchsummary", "haiku_query_failed", map[string]any{
			"error": err.Error(),
		})
		return "", ErrHaikuCallFailed
	}
	if result == nil || strings.TrimSpace(result.Text) == "" {
		return "No response from model", nil
	}

	logger.DebugCF("webfetchsummary", "summarize_done", map[string]any{
		"text_len": len(result.Text),
	})

	return result.Text, nil
}

// Summarize dispatches the request through the active package-level service.
// Returns ("", nil) when the service is uninitialised.
func Summarize(ctx context.Context, content string, prompt string, isPreapprovedDomain bool) (string, error) {
	svc := currentService()
	if svc == nil {
		return "", nil
	}
	return svc.Summarize(ctx, content, prompt, isPreapprovedDomain)
}
