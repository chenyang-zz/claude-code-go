package claudeailimits

import (
	"errors"
	"net/http"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/model"
	"github.com/sheepzhao/claude-code-go/internal/platform/oauth"
)

func TestInitRegistersStoreAndLoader(t *testing.T) {
	store := &fakeStore{}
	loader := SubscriptionLoaderFunc(func() (*oauth.OAuthTokens, error) {
		return &oauth.OAuthTokens{SubscriptionType: oauth.SubscriptionMax}, nil
	})

	Init(InitOptions{Store: store, SubscriptionLoader: loader})
	t.Cleanup(func() {
		Init(InitOptions{Store: nil, SubscriptionLoader: nil})
	})

	if got := getSettingsStore(); got != store {
		t.Fatal("Init did not register settings store")
	}
	if !IsClaudeAISubscriber() {
		t.Fatal("Init did not register subscription loader")
	}
}

func TestMakeAnthropicConsumerNilWhenFlagDisabled(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_CLAUDEAI_LIMITS", "0")
	if got := MakeAnthropicConsumer(); got != nil {
		t.Fatal("expected nil consumer when flag disabled")
	}
}

func TestMakeAnthropicConsumerSkipsTransportErrors(t *testing.T) {
	store := &fakeStore{}
	Init(InitOptions{Store: store, SubscriptionLoader: SubscriptionLoaderFunc(func() (*oauth.OAuthTokens, error) {
		return &oauth.OAuthTokens{SubscriptionType: oauth.SubscriptionMax}, nil
	})})
	t.Cleanup(func() { Init(InitOptions{Store: nil, SubscriptionLoader: nil}) })

	consumer := MakeAnthropicConsumer()
	if consumer == nil {
		t.Fatal("expected consumer")
	}
	consumer(nil, 0, errors.New("network down"))
	if store.saved != nil {
		t.Fatalf("expected save to be skipped on transport error, got %+v", store.saved)
	}
}

func TestMakeAnthropicConsumerSkipsNonSubscriber(t *testing.T) {
	store := &fakeStore{}
	Init(InitOptions{Store: store, SubscriptionLoader: nil})
	t.Cleanup(func() { Init(InitOptions{Store: nil, SubscriptionLoader: nil}) })

	consumer := MakeAnthropicConsumer()
	if consumer == nil {
		t.Fatal("expected consumer")
	}
	headers := http.Header{}
	headers.Set("anthropic-ratelimit-unified-status", "allowed")
	consumer(headers, 200, nil)
	if store.saved != nil {
		t.Fatalf("expected save to be skipped for non-subscriber, got %+v", store.saved)
	}
}

func TestMakeAnthropicConsumerPersistsForSubscriber(t *testing.T) {
	store := &fakeStore{}
	loader := SubscriptionLoaderFunc(func() (*oauth.OAuthTokens, error) {
		return &oauth.OAuthTokens{SubscriptionType: oauth.SubscriptionMax}, nil
	})
	Init(InitOptions{Store: store, SubscriptionLoader: loader})
	t.Cleanup(func() { Init(InitOptions{Store: nil, SubscriptionLoader: nil}) })

	consumer := MakeAnthropicConsumer()
	headers := http.Header{}
	headers.Set("anthropic-ratelimit-unified-status", "allowed")
	consumer(headers, 200, nil)
	if store.saved == nil {
		t.Fatal("expected save")
	}
	if status, _ := store.saved["status"].(string); status != "allowed" {
		t.Fatalf("expected status allowed, got %q", status)
	}
}

func TestMakeAnthropicConsumerForcesRejectedOn429(t *testing.T) {
	store := &fakeStore{}
	loader := SubscriptionLoaderFunc(func() (*oauth.OAuthTokens, error) {
		return &oauth.OAuthTokens{SubscriptionType: oauth.SubscriptionMax}, nil
	})
	Init(InitOptions{Store: store, SubscriptionLoader: loader})
	t.Cleanup(func() { Init(InitOptions{Store: nil, SubscriptionLoader: nil}) })

	consumer := MakeAnthropicConsumer()
	headers := http.Header{}
	// Headers say allowed_warning but the response was 429 — consumer
	// should override to rejected.
	headers.Set("anthropic-ratelimit-unified-status", "allowed_warning")
	headers.Set("anthropic-ratelimit-unified-representative-claim", "five_hour")
	consumer(headers, 429, nil)
	if store.saved == nil {
		t.Fatal("expected save")
	}
	if status, _ := store.saved["status"].(string); status != "rejected" {
		t.Fatalf("expected status rejected on 429, got %q", status)
	}
}

func TestMakeErrorAnnotatorNilWhenFlagDisabled(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_CLAUDEAI_LIMITS", "0")
	if got := MakeErrorAnnotator(); got != nil {
		t.Fatal("expected nil annotator when flag disabled")
	}
}

func TestMakeErrorAnnotatorReturnsNilForNilError(t *testing.T) {
	store := &fakeStore{loadVal: map[string]any{
		"status":        "rejected",
		"rateLimitType": "five_hour",
	}}
	loader := SubscriptionLoaderFunc(func() (*oauth.OAuthTokens, error) {
		return &oauth.OAuthTokens{SubscriptionType: oauth.SubscriptionPro}, nil
	})
	Init(InitOptions{Store: store, SubscriptionLoader: loader})
	t.Cleanup(func() { Init(InitOptions{Store: nil, SubscriptionLoader: nil}) })

	annotator := MakeErrorAnnotator()
	if got := annotator(nil, "claude-sonnet-4-6"); got != nil {
		t.Fatalf("expected nil for nil err, got %v", got)
	}
}

func TestMakeErrorAnnotatorReturnsNilForNonSubscriber(t *testing.T) {
	store := &fakeStore{loadVal: map[string]any{"status": "rejected"}}
	Init(InitOptions{Store: store, SubscriptionLoader: nil})
	t.Cleanup(func() { Init(InitOptions{Store: nil, SubscriptionLoader: nil}) })

	annotator := MakeErrorAnnotator()
	original := errors.New("boom")
	if got := annotator(original, "claude-sonnet-4-6"); got != nil {
		t.Fatalf("expected nil for non-subscriber, got %v", got)
	}
}

func TestMakeErrorAnnotatorReplacesMessageOnRejection(t *testing.T) {
	store := &fakeStore{loadVal: map[string]any{
		"status":        "rejected",
		"rateLimitType": "five_hour",
	}}
	loader := SubscriptionLoaderFunc(func() (*oauth.OAuthTokens, error) {
		return &oauth.OAuthTokens{SubscriptionType: oauth.SubscriptionPro}, nil
	})
	Init(InitOptions{Store: store, SubscriptionLoader: loader})
	t.Cleanup(func() { Init(InitOptions{Store: nil, SubscriptionLoader: nil}) })

	annotator := MakeErrorAnnotator()
	original := model.NewProviderError(model.ProviderErrorRateLimit, "anthropic", http.StatusTooManyRequests, "api error 429")
	got := annotator(original, "claude-sonnet-4-6")
	if got == nil {
		t.Fatal("expected non-nil annotated error")
	}

	annotated, ok := got.(*AnnotatedError)
	if !ok {
		t.Fatalf("expected *AnnotatedError, got %T", got)
	}
	if annotated.Message == "" {
		t.Fatal("expected annotated message")
	}
	if !errors.Is(annotated, original) {
		t.Fatal("annotated error should wrap the original via Unwrap")
	}
}

func TestMakeErrorAnnotatorIgnoresNonRateLimitErrors(t *testing.T) {
	// Stale snapshot says rejected but the current error is a 5xx server
	// failure. Annotator must leave the original error intact so users
	// see the real failure mode.
	store := &fakeStore{loadVal: map[string]any{
		"status":        "rejected",
		"rateLimitType": "five_hour",
	}}
	loader := SubscriptionLoaderFunc(func() (*oauth.OAuthTokens, error) {
		return &oauth.OAuthTokens{SubscriptionType: oauth.SubscriptionPro}, nil
	})
	Init(InitOptions{Store: store, SubscriptionLoader: loader})
	t.Cleanup(func() { Init(InitOptions{Store: nil, SubscriptionLoader: nil}) })

	annotator := MakeErrorAnnotator()
	serverErr := model.NewProviderError(model.ProviderErrorServerError, "anthropic", http.StatusInternalServerError, "boom")
	if got := annotator(serverErr, "claude-sonnet-4-6"); got != nil {
		t.Fatalf("expected nil for non-rate-limit error, got %v", got)
	}
	plainErr := errors.New("network blip")
	if got := annotator(plainErr, "claude-sonnet-4-6"); got != nil {
		t.Fatalf("expected nil for plain error, got %v", got)
	}
}

func TestMakeErrorAnnotatorReturnsNilWhenLimitsAllowed(t *testing.T) {
	store := &fakeStore{loadVal: map[string]any{"status": "allowed"}}
	loader := SubscriptionLoaderFunc(func() (*oauth.OAuthTokens, error) {
		return &oauth.OAuthTokens{SubscriptionType: oauth.SubscriptionPro}, nil
	})
	Init(InitOptions{Store: store, SubscriptionLoader: loader})
	t.Cleanup(func() { Init(InitOptions{Store: nil, SubscriptionLoader: nil}) })

	annotator := MakeErrorAnnotator()
	if got := annotator(errors.New("network blip"), "claude-sonnet-4-6"); got != nil {
		t.Fatalf("expected nil when limits.Status is allowed, got %v", got)
	}
}

func TestAnnotatedErrorMessage(t *testing.T) {
	original := errors.New("api error 429")
	wrapped := &AnnotatedError{Underlying: original, Message: "You've hit your session limit"}
	if wrapped.Error() != "You've hit your session limit" {
		t.Fatalf("Error() = %q", wrapped.Error())
	}
	if !errors.Is(wrapped, original) {
		t.Fatal("Unwrap should expose original error")
	}
}

func TestAnnotatedErrorEmptyMessageFallsBack(t *testing.T) {
	original := errors.New("api error 429")
	wrapped := &AnnotatedError{Underlying: original}
	if wrapped.Error() != "api error 429" {
		t.Fatalf("Error() = %q, want underlying", wrapped.Error())
	}
}

func TestLoadOAuthTokensFromStoreNil(t *testing.T) {
	if got := LoadOAuthTokensFromStore(nil); got != nil {
		t.Fatal("expected nil loader for nil store")
	}
}
