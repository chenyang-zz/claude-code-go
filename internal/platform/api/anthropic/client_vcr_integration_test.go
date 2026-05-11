// Package anthropic integration tests with VCR recording/replay.
//
// These tests demonstrate how to use the VCR service to record real API
// interactions and replay them deterministically in CI.
//
// Usage:
//
//	# Passthrough (default): tests are skipped
//	go test ./internal/platform/api/anthropic/ -run TestVCR -v
//
//	# Record mode: makes real API calls, saves fixtures
//	VCR_RECORD=true \
//	  ANTHROPIC_API_KEY=sk-... \
//	  ANTHROPIC_BASE_URL=https://your-gateway/ \
//	  ANTHROPIC_MODEL=deepseek-v4-pro \
//	  go test ./internal/platform/api/anthropic/ -run TestVCR -v
//
//	# Replay mode: uses fixtures, no network calls
//	VCR_ENABLED=true \
//	  ANTHROPIC_MODEL=deepseek-v4-pro \
//	  go test ./internal/platform/api/anthropic/ -run TestVCR -v
//
// Environment variables:
//   - ANTHROPIC_API_KEY: API key for recording (not needed in replay)
//   - ANTHROPIC_BASE_URL: optional API gateway URL override
//   - ANTHROPIC_MODEL: model name (default: claude-sonnet-4-5-20250514)
//   - CLAUDE_CODE_TEST_FIXTURES_ROOT: should point to project root for stable
//     fixture paths (default: current working directory)

package anthropic

import (
	"context"
	"os"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
	"github.com/sheepzhao/claude-code-go/internal/core/model"
	"github.com/sheepzhao/claude-code-go/internal/platform/vcr"
)

// vcrBaseURL returns the API base URL from ANTHROPIC_BASE_URL env var,
// or empty string (meaning use the SDK default).
func vcrBaseURL() string {
	return os.Getenv("ANTHROPIC_BASE_URL")
}

// vcrModel returns the model name from ANTHROPIC_MODEL env var,
// or the default "claude-sonnet-4-5-20250514".
func vcrModel() string {
	if m := os.Getenv("ANTHROPIC_MODEL"); m != "" {
		return m
	}
	return "claude-sonnet-4-5-20250514"
}

// vcrEnabledAndSkipped returns true and skips the test when VCR is not active.
func vcrEnabledAndSkipped(t *testing.T) bool {
	t.Helper()
	if !vcr.Enabled() && !vcr.Recording() {
		t.Skip("VCR not enabled; set VCR_ENABLED=true or VCR_RECORD=true")
		return true
	}
	return false
}

// TestVCRStreamTextResponse records/replays a simple text streaming interaction.
// Verifies that text delta events are correctly captured and replayed.
func TestVCRStreamTextResponse(t *testing.T) {
	if vcrEnabledAndSkipped(t) {
		return
	}

	apiKey := os.Getenv("ANTHROPIC_API_KEY")

	inner := NewClient(Config{
		APIKey:  apiKey,
		BaseURL: vcrBaseURL(),
	})

	wrapped := vcr.WrapModelClient("text-response", inner)

	ctx := context.Background()
	stream, err := wrapped.Stream(ctx, model.Request{
		Model: vcrModel(),
		Messages: []message.Message{
			{
				Role:    message.RoleUser,
				Content: []message.ContentPart{message.TextPart("Reply with exactly one word: hello")},
			},
		},
		MaxOutputTokens: 50,
	})
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	var gotText string
	var gotUsage *model.Usage
	for evt := range stream {
		switch evt.Type {
		case model.EventTypeTextDelta:
			gotText += evt.Text
		case model.EventTypeDone:
			gotUsage = evt.Usage
		case model.EventTypeError:
			t.Fatalf("Stream error: %s", evt.Error)
		}
	}

	if gotText == "" {
		t.Error("expected non-empty text response")
	}
	if gotUsage == nil {
		t.Error("expected usage in done event")
	} else {
		t.Logf("Input tokens: %d, Output tokens: %d", gotUsage.InputTokens, gotUsage.OutputTokens)
	}
	t.Logf("Model response: %s", gotText)
}

// TestVCRStreamWithTools records/replays a tool-use interaction.
// Verifies that tool_use blocks are correctly captured and replayed.
func TestVCRStreamWithTools(t *testing.T) {
	if vcrEnabledAndSkipped(t) {
		return
	}

	apiKey := os.Getenv("ANTHROPIC_API_KEY")

	inner := NewClient(Config{
		APIKey:  apiKey,
		BaseURL: vcrBaseURL(),
	})

	wrapped := vcr.WrapModelClient("tool-use", inner)

	ctx := context.Background()
	stream, err := wrapped.Stream(ctx, model.Request{
		Model: vcrModel(),
		Messages: []message.Message{
			{
				Role:    message.RoleUser,
				Content: []message.ContentPart{message.TextPart("What is 2+2? Use the calculator tool.")},
			},
		},
		Tools: []model.ToolDefinition{
			{
				Name:        "calculator",
				Description: "Evaluate a mathematical expression",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"expression": map[string]any{
							"type":        "string",
							"description": "The math expression to evaluate",
						},
					},
					"required": []string{"expression"},
				},
			},
		},
		MaxOutputTokens: 200,
	})
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	var toolUses int
	var gotText string
	for evt := range stream {
		switch evt.Type {
		case model.EventTypeTextDelta:
			gotText += evt.Text
		case model.EventTypeToolUse:
			toolUses++
			t.Logf("Tool use: %s(%s)", evt.ToolUse.Name, evt.ToolUse.ID)
			t.Logf("  Input: %v", evt.ToolUse.Input)
		case model.EventTypeDone:
			t.Logf("Stop reason: %s", evt.StopReason)
		case model.EventTypeError:
			t.Fatalf("Stream error: %s", evt.Error)
		}
	}

	if gotText == "" && toolUses == 0 {
		t.Error("expected either text or tool_use in response")
	}
}

// TestVCRStreamWithSystemPrompt records/replays a streaming interaction
// that includes a system prompt, verifying system messages are preserved.
func TestVCRStreamWithSystemPrompt(t *testing.T) {
	if vcrEnabledAndSkipped(t) {
		return
	}

	apiKey := os.Getenv("ANTHROPIC_API_KEY")

	inner := NewClient(Config{
		APIKey:  apiKey,
		BaseURL: vcrBaseURL(),
	})

	wrapped := vcr.WrapModelClient("system-prompt", inner)

	ctx := context.Background()
	stream, err := wrapped.Stream(ctx, model.Request{
		Model: vcrModel(),
		System: "You are a helpful assistant. Always respond in JSON format with a single 'answer' field.",
		Messages: []message.Message{
			{
				Role:    message.RoleUser,
				Content: []message.ContentPart{message.TextPart("What is the capital of France?")},
			},
		},
		MaxOutputTokens: 100,
	})
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	var gotText string
	for evt := range stream {
		switch evt.Type {
		case model.EventTypeTextDelta:
			gotText += evt.Text
		case model.EventTypeDone:
			t.Logf("Response complete (stop: %s)", evt.StopReason)
			if evt.Usage != nil {
				t.Logf("Usage — input: %d, output: %d",
					evt.Usage.InputTokens, evt.Usage.OutputTokens)
			}
		case model.EventTypeError:
			t.Fatalf("Stream error: %s", evt.Error)
		}
	}

	if gotText == "" {
		t.Error("expected non-empty response")
	}
	t.Logf("Response: %s", gotText)
}
