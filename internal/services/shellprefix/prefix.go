// Package shellprefix extracts shell command prefixes via the Haiku helper.
// The package mirrors src/utils/shell/prefix.ts: given a user command and
// policy specification, it calls Haiku to determine the leading prefix (e.g.
// "python3", "npm" — or "none" / "command_injection_detected"). Dangerous
// shell executables and the bare "git" prefix are rejected.
package shellprefix

import (
	"context"
	"errors"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/services/haiku"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// ErrHaikuCallFailed is returned when the Haiku query itself fails and the
// caller should fall back to a safe default (treat as "no prefix available").
var ErrHaikuCallFailed = errors.New("shellprefix: haiku query failed")

// Service holds a haiku.Querier used to dispatch command prefix extraction
// requests.
type Service struct {
	querier haiku.Querier
}

// NewService constructs a Service bound to the given Querier. Returns nil when
// querier is nil so callers can detect an unwired runtime.
func NewService(querier haiku.Querier) *Service {
	if querier == nil {
		return nil
	}
	return &Service{querier: querier}
}

// Extract calls Haiku to determine the commanding prefix for the given shell
// command. The policySpec provides tool-specific examples and instructions that
// Haiku uses to classify the prefix.
//
// Return values mirror the TS three-state model:
//   - (non-empty string, nil): a valid prefix was extracted
//   - ("", nil): no prefix could be determined (equivalent to TS null)
//   - ("", error): the Haiku call itself failed
//
// Returns ("", nil) when the service is nil, the feature flag is disabled,
// the command is empty, or ctx is cancelled.
func (s *Service) Extract(ctx context.Context, command string, policySpec string) (string, error) {
	if s == nil || s.querier == nil {
		return "", nil
	}
	if !IsShellPrefixEnabled() {
		return "", nil
	}
	trimmed := strings.TrimSpace(command)
	if trimmed == "" {
		return "", nil
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}

	logger.DebugCF("shellprefix", "extract_start", map[string]any{
		"command_len": len(trimmed),
	})

	result, err := s.querier.Query(ctx, haiku.QueryParams{
		SystemPrompt:       systemPrompt,
		UserPrompt:         policySpec + "\n\nCommand: " + trimmed,
		MaxOutputTokens:    64,
		QuerySource:        querySource,
	})
	if err != nil {
		logger.DebugCF("shellprefix", "haiku_query_failed", map[string]any{
			"error": err.Error(),
		})
		return "", ErrHaikuCallFailed
	}
	if result == nil {
		return "", nil
	}

	prefix := strings.TrimSpace(result.Text)
	if prefix == "" {
		return "", nil
	}

	// Classify the prefix against the TS result routing table.
	processed := classifyPrefix(prefix, trimmed)

	logger.DebugCF("shellprefix", "extract_done", map[string]any{
		"prefix":  processed,
		"success": processed != "",
	})

	return processed, nil
}

// classifyPrefix applies the TS-side result classification rules to the raw
// Haiku response. Returns "" when the prefix is rejected (dangerous, "none",
// injection, mismatch) and the validated prefix otherwise.
func classifyPrefix(rawPrefix string, command string) string {
	// Check for command injection detection first.
	if rawPrefix == "command_injection_detected" {
		return ""
	}

	// Reject bare "git" — never accept as a prefix.
	if rawPrefix == "git" {
		return ""
	}

	// Reject dangerous shell executables.
	if dangerousShellPrefixes.has(rawPrefix) {
		return ""
	}

	// "none" means Haiku found no meaningful prefix.
	if rawPrefix == "none" {
		return ""
	}

	// Validate that the prefix is actually a prefix of the command.
	if !strings.HasPrefix(command, rawPrefix) {
		return ""
	}

	return rawPrefix
}

// Generate dispatches the request through the active package-level service.
// Returns ("", nil) when the service is uninitialised.
func Generate(ctx context.Context, command string, policySpec string) (string, error) {
	svc := currentService()
	if svc == nil {
		return "", nil
	}
	return svc.Extract(ctx, command, policySpec)
}
