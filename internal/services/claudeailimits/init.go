package claudeailimits

import (
	"net/http"

	"github.com/sheepzhao/claude-code-go/internal/platform/oauth"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// InitOptions captures the wiring inputs required to bootstrap the
// claudeailimits service.
type InitOptions struct {
	// Store is the global settings persistence backend. Required for the
	// SaveClaudeAILimits / LoadClaudeAILimits helpers.
	Store SettingsStore
	// SubscriptionLoader returns the current OAuth tokens. Used by the
	// gating helpers (IsClaudeAISubscriber and friends).
	SubscriptionLoader SubscriptionLoader
}

// Init wires the claudeailimits service with the supplied dependencies.
// Safe to call multiple times — the most recent options win. Calling Init
// with nil options short-circuits to a no-op so bootstrap can defer wiring
// without crashing dependents.
func Init(opts InitOptions) {
	SetSettingsStore(opts.Store)
	SetSubscriptionLoader(opts.SubscriptionLoader)
	logger.DebugCF("claudeailimits", "initialised", map[string]any{
		"store_configured":      opts.Store != nil,
		"subscription_loader":   opts.SubscriptionLoader != nil,
		"feature_flag_enabled":  IsClaudeAILimitsEnabled(),
	})
}

// MakeAnthropicConsumer returns a callback compatible with
// `anthropic.Config.RateLimitConsumer`. The callback projects each response
// header set onto the persisted ClaudeAILimits state and (when 429 status is
// observed) forces the snapshot status to rejected so the runtime can
// surface the right user-facing message.
//
// Returns nil when the feature flag is disabled so callers can safely set
// the field unconditionally.
func MakeAnthropicConsumer() func(http.Header, int, error) {
	if !IsClaudeAILimitsEnabled() {
		return nil
	}

	return func(headers http.Header, status int, err error) {
		// Skip transport errors and missing headers; the caller surfaces
		// the network failure separately.
		if err != nil || headers == nil {
			return
		}
		// Skip when no subscriber is logged in. This mirrors the TS
		// `shouldProcessRateLimits` gate and avoids polluting api-key
		// users' settings with claude.ai data.
		if !IsClaudeAISubscriber() {
			return
		}

		limits := ProcessRateLimitHeaders(headers)
		if limits == nil {
			return
		}

		// Mirror the TS extractQuotaStatusFromError fallback: a 429
		// always means rejected even if headers were not present.
		if status == http.StatusTooManyRequests {
			limits.Status = QuotaStatusRejected
		}

		if saveErr := SaveClaudeAILimits(limits); saveErr != nil {
			logger.WarnCF("claudeailimits", "failed to persist rate limits", map[string]any{
				"error": saveErr.Error(),
			})
		}
	}
}

// MakeErrorAnnotator returns a callback compatible with
// `anthropic.Config.RateLimitErrorAnnotator`. The callback inspects the
// supplied provider error and, when it represents a rate-limit rejection
// for a Claude.ai subscriber, replaces the message with a user-friendly
// limit-reached sentence rendered from the persisted ClaudeAILimits state.
//
// Returns nil when the feature flag is disabled so callers can safely set
// the field unconditionally.
func MakeErrorAnnotator() func(err error, modelName string) error {
	if !IsClaudeAILimitsEnabled() {
		return nil
	}
	return func(err error, modelName string) error {
		if err == nil {
			return nil
		}
		if !IsClaudeAISubscriber() {
			return nil
		}
		// Only annotate if the persisted snapshot indicates we are rate
		// limited; otherwise leave the original provider error verbatim
		// so the caller surfaces the most accurate diagnostic.
		limits, loadErr := LoadClaudeAILimits()
		if loadErr != nil || limits == nil {
			return nil
		}
		message := GetRateLimitErrorMessage(limits, modelName)
		if message == "" {
			return nil
		}
		return &AnnotatedError{Underlying: err, Message: message}
	}
}

// AnnotatedError wraps an upstream provider error with a user-facing message
// rendered from the persisted ClaudeAILimits snapshot. The underlying error
// is preserved so retry / fallback / circuit-breaker decisions remain
// unchanged; callers that surface the message to humans should use the
// `Message` field directly.
type AnnotatedError struct {
	// Underlying is the original provider error returned by the upstream
	// client. Kept so errors.Is / errors.As can match against the existing
	// ProviderError sentinels.
	Underlying error
	// Message is the user-friendly limit-reached sentence rendered from
	// the persisted ClaudeAILimits state.
	Message string
}

// Error returns the human-friendly message so callers that print the error
// directly (Console UI, log lines, status footer) get the expected text.
func (e *AnnotatedError) Error() string {
	if e == nil || e.Message == "" {
		if e != nil && e.Underlying != nil {
			return e.Underlying.Error()
		}
		return ""
	}
	return e.Message
}

// Unwrap exposes the wrapped error so errors.Is and errors.As can reach the
// original ProviderError.
func (e *AnnotatedError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Underlying
}

// LoadOAuthTokensFromStore is a convenience adapter that wraps an
// `*oauth.OAuthCredentialStore` into a `SubscriptionLoader`. Bootstrap
// passes the active credential store so the gating helpers can answer
// without an extra dependency.
func LoadOAuthTokensFromStore(store *oauth.OAuthCredentialStore) SubscriptionLoader {
	if store == nil {
		return nil
	}
	return SubscriptionLoaderFunc(func() (*oauth.OAuthTokens, error) {
		return store.Load()
	})
}
