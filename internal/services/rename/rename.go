package rename

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
	"github.com/sheepzhao/claude-code-go/internal/services/haiku"
	"github.com/sheepzhao/claude-code-go/internal/services/sessiontitle"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// Service holds a haiku.Querier used to dispatch rename suggestion requests.
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

// Suggest sends a rename suggestion request through the bound haiku querier.
// Returns ("", nil) for any of the following non-error paths so the caller
// can treat the suggestion as best-effort:
//   - feature flag disabled
//   - empty messages / no extractable text
//   - haiku query returns an error (logged)
//   - haiku query returns empty or unparseable text
//
// All hard errors are absorbed; this matches the TS catch -> return null semantics.
func (s *Service) Suggest(ctx context.Context, messages []message.Message) (string, error) {
	if s == nil || s.querier == nil {
		return "", nil
	}
	if !IsRenameSuggestionEnabled() {
		return "", nil
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}

	conversationText := sessiontitle.ExtractConversationText(messages)
	if conversationText == "" {
		return "", nil
	}

	result, err := s.querier.Query(ctx, haiku.QueryParams{
		SystemPrompt:            systemPrompt,
		UserPrompt:              conversationText,
		EnablePromptCaching:     true,
		IsNonInteractiveSession: false,
		QuerySource:             querySource,
	})
	if err != nil {
		logger.DebugCF("rename", "haiku_query_failed", map[string]any{
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
		Name string `json:"name"`
	}
	if err := json.Unmarshal([]byte(text), &parsed); err != nil {
		logger.DebugCF("rename", "json_unmarshal_failed", map[string]any{
			"error": err.Error(),
			"text":  text,
		})
		return "", nil
	}

	name := strings.TrimSpace(parsed.Name)
	if name == "" {
		return "", nil
	}
	return name, nil
}

// Suggest dispatches the request through the active package-level service.
// Returns ("", nil) when the service is uninitialised.
func Suggest(ctx context.Context, messages []message.Message) (string, error) {
	svc := currentService()
	if svc == nil {
		return "", nil
	}
	return svc.Suggest(ctx, messages)
}
