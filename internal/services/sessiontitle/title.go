package sessiontitle

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/services/haiku"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// Service holds a haiku.Querier used to dispatch session title requests.
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

// Generate sends a session title request through the bound haiku querier.
// Returns ("", nil) for any of the following non-error paths so the caller
// can treat the title as best-effort:
//   - feature flag disabled
//   - empty description
//   - haiku query returns an error (logged)
//   - haiku query returns empty or unparseable text
//
// All hard errors are absorbed; this matches the TS catch -> return null semantics.
func (s *Service) Generate(ctx context.Context, description string) (string, error) {
	if s == nil || s.querier == nil {
		return "", nil
	}
	if !IsSessionTitleEnabled() {
		return "", nil
	}
	trimmed := strings.TrimSpace(description)
	if trimmed == "" {
		return "", nil
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}

	result, err := s.querier.Query(ctx, haiku.QueryParams{
		SystemPrompt:            systemPrompt,
		UserPrompt:              trimmed,
		EnablePromptCaching:     true,
		IsNonInteractiveSession: false,
		QuerySource:             querySource,
	})
	if err != nil {
		logger.DebugCF("session_title", "haiku_query_failed", map[string]any{
			"error": err.Error(),
		})
		return "", nil
	}
	if result == nil {
		return "", nil
	}

	text := strings.TrimSpace(result.Text)
	if text == "" {
		return "", nil
	}

	var parsed struct {
		Title string `json:"title"`
	}
	if err := json.Unmarshal([]byte(text), &parsed); err != nil {
		logger.DebugCF("session_title", "json_unmarshal_failed", map[string]any{
			"error": err.Error(),
			"text":  text,
		})
		return "", nil
	}

	title := strings.TrimSpace(parsed.Title)
	if title == "" {
		return "", nil
	}
	return title, nil
}

// Generate dispatches the request through the active package-level service.
// Returns ("", nil) when the service is uninitialised.
func Generate(ctx context.Context, description string) (string, error) {
	svc := currentService()
	if svc == nil {
		return "", nil
	}
	return svc.Generate(ctx, description)
}
