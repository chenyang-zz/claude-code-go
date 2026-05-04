package datetimeparser

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/services/haiku"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// DateTimeParseResult is the discriminated union returned by Parse.
// When Success is true, Value carries the ISO 8601 string.
// When Success is false, Error carries a human-readable message.
type DateTimeParseResult struct {
	Success bool
	Value   string
	Error   string
}

// Service holds a haiku.Querier used to dispatch date/time parsing requests.
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

// Parse sends a natural-language date/time parsing request through the bound
// haiku querier. Returns a DateTimeParseResult for all paths; errors are
// absorbed into the result's Error field rather than returned as Go errors.
//
// The format argument should be either "date" or "date-time", matching the
// TS 'date' | 'date-time' union.
func (s *Service) Parse(ctx context.Context, input string, format string) (DateTimeParseResult, error) {
	if s == nil || s.querier == nil {
		return DateTimeParseResult{Success: false, Error: "service unavailable"}, nil
	}
	if !IsDateTimeParserEnabled() {
		return DateTimeParseResult{Success: false, Error: "datetime parser disabled"}, nil
	}
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return DateTimeParseResult{Success: false, Error: "empty input"}, nil
	}
	if err := ctx.Err(); err != nil {
		return DateTimeParseResult{}, err
	}

	// Build time context (mirrors dateTimeParser.ts lines 29-36).
	now := time.Now()
	currentDateTime := now.UTC().Format(time.RFC3339Nano)
	_, offsetSec := now.Zone()
	offsetMin := offsetSec / 60
	sign := '+'
	if offsetMin < 0 {
		sign = '-'
		offsetMin = -offsetMin
	}
	hours := offsetMin / 60
	minutes := offsetMin % 60
	offsetStr := fmt.Sprintf("%c%02d:%02d", sign, hours, minutes)
	dayOfWeek := now.Weekday().String()

	formatDescription := dateFormatDescription
	if format == "date-time" {
		formatDescription = fmt.Sprintf(dateTimeFormatDescriptionTemplate, offsetStr)
	}

	userPrompt := fmt.Sprintf(userPromptTemplate, currentDateTime, offsetStr, dayOfWeek, trimmed, formatDescription)

	result, err := s.querier.Query(ctx, haiku.QueryParams{
		SystemPrompt:            systemPrompt,
		UserPrompt:              userPrompt,
		EnablePromptCaching:     false,
		IsNonInteractiveSession: false,
		QuerySource:             querySource,
	})
	if err != nil {
		logger.DebugCF("datetime_parser", "haiku_query_failed", map[string]any{
			"error": err.Error(),
		})
		return DateTimeParseResult{
			Success: false,
			Error:   "Unable to parse date/time. Please enter in ISO 8601 format manually.",
		}, nil
	}
	if result == nil {
		return DateTimeParseResult{
			Success: false,
			Error:   "Unable to parse date/time from input",
		}, nil
	}

	parsedText := strings.TrimSpace(result.Text)
	if parsedText == "" || parsedText == "INVALID" {
		return DateTimeParseResult{
			Success: false,
			Error:   "Unable to parse date/time from input",
		}, nil
	}

	// Basic sanity check — should start with a 4-digit year.
	if len(parsedText) < 4 || !isDigit(parsedText[0]) || !isDigit(parsedText[1]) || !isDigit(parsedText[2]) || !isDigit(parsedText[3]) {
		return DateTimeParseResult{
			Success: false,
			Error:   "Unable to parse date/time from input",
		}, nil
	}

	return DateTimeParseResult{Success: true, Value: parsedText}, nil
}

// Parse dispatches the request through the active package-level service.
// Returns an error-result when the service is uninitialised.
func Parse(ctx context.Context, input string, format string) (DateTimeParseResult, error) {
	svc := currentService()
	if svc == nil {
		return DateTimeParseResult{Success: false, Error: "service unavailable"}, nil
	}
	return svc.Parse(ctx, input, format)
}

// looksLikeISO8601 reports whether the input appears to be an ISO 8601
// date or date-time string. Used by callers to decide whether to attempt
// natural-language parsing.
func looksLikeISO8601(input string) bool {
	trimmed := strings.TrimSpace(input)
	if len(trimmed) < 10 {
		return false
	}
	// Match YYYY-MM-DD or YYYY-MM-DDTHH:MM:SS...
	for i := 0; i < 4; i++ {
		if !isDigit(trimmed[i]) {
			return false
		}
	}
	if trimmed[4] != '-' {
		return false
	}
	if !isDigit(trimmed[5]) || !isDigit(trimmed[6]) {
		return false
	}
	if trimmed[7] != '-' {
		return false
	}
	if !isDigit(trimmed[8]) || !isDigit(trimmed[9]) {
		return false
	}
	return len(trimmed) == 10 || trimmed[10] == 'T'
}

func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}
