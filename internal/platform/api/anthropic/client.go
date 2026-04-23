package anthropic

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
	"github.com/sheepzhao/claude-code-go/internal/core/model"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const defaultBaseURL = "https://api.anthropic.com"

// Config carries the minimum Anthropic client configuration needed by batch-07.
type Config struct {
	// APIKey carries the Anthropic credential.
	APIKey string
	// AuthToken carries the Anthropic bearer token when first-party account auth is used.
	AuthToken string
	// BaseURL optionally overrides the default Anthropic API host.
	BaseURL string
	// HTTPClient allows tests to inject a local transport.
	HTTPClient *http.Client
	// IsFirstParty indicates whether this client connects to the first-party
	// Anthropic API (as opposed to Vertex AI or Bedrock). First-party-only
	// beta headers like task-budgets are only included when true.
	IsFirstParty bool
}

// Client implements the minimum Anthropic SSE text stream client used by the runtime engine.
type Client struct {
	// apiKey stores the request credential.
	apiKey string
	// authToken stores the bearer token credential when first-party account auth is used.
	authToken string
	// baseURL stores the API host root.
	baseURL string
	// httpClient performs HTTP requests.
	httpClient *http.Client
	// isFirstParty indicates first-party Anthropic API for beta header gating.
	isFirstParty bool
}

type messagesRequest struct {
	Model       string             `json:"model"`
	MaxTokens   int                `json:"max_tokens"`
	System      string             `json:"system,omitempty"`
	Stream      bool               `json:"stream"`
	Messages    []anthropicMessage `json:"messages"`
	Tools       []anthropicTool    `json:"tools,omitempty"`
	OutputConfig *anthropicOutputConfig `json:"output_config,omitempty"`
}

type anthropicMessage struct {
	Role    string                  `json:"role"`
	Content []anthropicContentBlock `json:"content"`
}

type anthropicContentBlock struct {
	Type         string         `json:"type"`
	Text         string         `json:"text,omitempty"`
	Thinking     string         `json:"thinking,omitempty"`
	Signature    string         `json:"signature,omitempty"`
	Data         string         `json:"data,omitempty"`
	ID           string         `json:"id,omitempty"`
	Name         string         `json:"name,omitempty"`
	Input        map[string]any `json:"input,omitempty"`
	ToolUseID    string         `json:"tool_use_id,omitempty"`
	Content      any            `json:"content,omitempty"`
	IsError      bool           `json:"is_error,omitempty"`
	CacheControl *cacheControl  `json:"cache_control,omitempty"`
}

// cacheControl carries the Anthropic cache_control marker used for prompt caching.
type cacheControl struct {
	Type string `json:"type"`
}

type anthropicTool struct {
	Name         string         `json:"name"`
	Description  string         `json:"description,omitempty"`
	InputSchema  map[string]any `json:"input_schema"`
	CacheControl *cacheControl  `json:"cache_control,omitempty"`
}

// anthropicOutputConfig carries the output_config field sent in the API request body.
type anthropicOutputConfig struct {
	TaskBudget *anthropicTaskBudget `json:"task_budget,omitempty"`
}

// anthropicTaskBudget is the wire format for output_config.task_budget.
// See API schema api/api/schemas/messages/request/output_config.py.
type anthropicTaskBudget struct {
	Type      string `json:"type"`
	Total     int    `json:"total"`
	Remaining *int   `json:"remaining,omitempty"`
}

// taskBudgetsBetaHeader is the beta header that enables the task_budget feature.
const taskBudgetsBetaHeader = "task-budgets-2026-03-13"

type streamContentBlock struct {
	blockType string
	toolID    string
	toolName  string
	inputJSON strings.Builder
	thinking  strings.Builder
	signature string
	data      string
}

type contentBlockStartEnvelope struct {
	Index        int `json:"index"`
	ContentBlock struct {
		Type      string `json:"type"`
		ID        string `json:"id"`
		Name      string `json:"name"`
		Thinking  string `json:"thinking,omitempty"`
		Signature string `json:"signature,omitempty"`
		Data      string `json:"data,omitempty"`
	} `json:"content_block"`
}

type contentBlockDeltaEnvelope struct {
	Index int `json:"index"`
	Delta struct {
		Type        string `json:"type"`
		Text        string `json:"text"`
		Thinking    string `json:"thinking,omitempty"`
		Signature   string `json:"signature,omitempty"`
		PartialJSON string `json:"partial_json"`
	} `json:"delta"`
}

type contentBlockStopEnvelope struct {
	Index int `json:"index"`
}

type errorEnvelope struct {
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

// messageStartEnvelope captures usage from the Anthropic message_start event.
type messageStartEnvelope struct {
	Message struct {
		Usage struct {
			InputTokens              int `json:"input_tokens"`
			OutputTokens             int `json:"output_tokens"`
			CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
			CacheReadInputTokens     int `json:"cache_read_input_tokens"`
		} `json:"usage"`
	} `json:"message"`
}

// messageDeltaEnvelope captures output usage and stop_reason from the Anthropic message_delta event.
type messageDeltaEnvelope struct {
	Delta struct {
		StopReason string `json:"stop_reason"`
	} `json:"delta"`
	Usage struct {
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// streamState accumulates metadata parsed from Anthropic SSE events across the stream lifetime.
type streamState struct {
	inputTokens       int
	cacheCreateTokens int
	cacheReadTokens   int
	outputTokens      int
	stopReason        string
}

// NewClient builds a minimal Anthropic streaming client.
func NewClient(cfg Config) *Client {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	return &Client{
		apiKey:       cfg.APIKey,
		authToken:    cfg.AuthToken,
		baseURL:      strings.TrimRight(baseURL, "/"),
		httpClient:   httpClient,
		isFirstParty: cfg.IsFirstParty,
	}
}

// Stream opens one Anthropic streaming request and converts SSE payloads into model events.
func (c *Client) Stream(ctx context.Context, req model.Request) (model.Stream, error) {
	if c.apiKey == "" && c.authToken == "" {
		return nil, fmt.Errorf("missing Anthropic auth credential")
	}

	body, err := json.Marshal(c.buildMessagesRequest(req))
	if err != nil {
		return nil, fmt.Errorf("marshal anthropic request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build anthropic request: %w", err)
	}

	httpReq.Header.Set("content-type", "application/json")
	httpReq.Header.Set("accept", "text/event-stream")
	if c.apiKey != "" {
		httpReq.Header.Set("x-api-key", c.apiKey)
	}
	if c.authToken != "" {
		httpReq.Header.Set("authorization", "Bearer "+c.authToken)
	}
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	// Inject task-budgets beta header when task budget is configured
	// and this is a first-party Anthropic request.
	if req.TaskBudget != nil && c.isFirstParty {
		httpReq.Header.Set("anthropic-beta", taskBudgetsBetaHeader)
	}

	logger.DebugCF("anthropic_client", "starting anthropic stream", map[string]any{
		"model":         req.Model,
		"message_count": len(req.Messages),
		"base_url":      c.baseURL,
	})

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("execute anthropic request: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		payload, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("anthropic api error: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(payload)))
	}

	out := make(chan model.Event)
	go func() {
		defer close(out)
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

		var eventName string
		var dataLines []string
		contentBlocks := make(map[int]*streamContentBlock)
		var state streamState

		flush := func() {
			if len(dataLines) == 0 {
				eventName = ""
				return
			}

			c.handleSSEEvent(eventName, strings.Join(dataLines, "\n"), contentBlocks, &state, out)
			eventName = ""
			dataLines = dataLines[:0]
		}

		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				flush()
				continue
			}

			switch {
			case strings.HasPrefix(line, "event:"):
				eventName = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			case strings.HasPrefix(line, "data:"):
				dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
			}
		}
		flush()

		if err := scanner.Err(); err != nil {
			out <- model.Event{
				Type:  model.EventTypeError,
				Error: fmt.Sprintf("read anthropic stream: %v", err),
			}
			return
		}

		doneEvent := model.Event{Type: model.EventTypeDone}
		if state.stopReason != "" {
			doneEvent.StopReason = model.StopReason(state.stopReason)
		}
		if state.inputTokens > 0 || state.outputTokens > 0 || state.cacheCreateTokens > 0 || state.cacheReadTokens > 0 {
			doneEvent.Usage = &model.Usage{
				InputTokens:              state.inputTokens,
				OutputTokens:             state.outputTokens,
				CacheCreationInputTokens: state.cacheCreateTokens,
				CacheReadInputTokens:     state.cacheReadTokens,
			}
		}
		out <- doneEvent
	}()

	return out, nil
}

func maxOutputTokens(req model.Request) int {
	if req.MaxOutputTokens > 0 {
		return req.MaxOutputTokens
	}
	return 1024
}

// buildMessagesRequest constructs the wire-format request body from the shared model request.
func (c *Client) buildMessagesRequest(req model.Request) messagesRequest {
	msgReq := messagesRequest{
		Model:     req.Model,
		MaxTokens: maxOutputTokens(req),
		System:    req.System,
		Stream:    true,
		Messages:  mapMessages(req.Messages, req.EnablePromptCaching),
		Tools:     mapTools(req.Tools),
	}

	// Include output_config.task_budget when the caller provides one
	// and this is a first-party Anthropic request.
	if req.TaskBudget != nil && c.isFirstParty {
		msgReq.OutputConfig = &anthropicOutputConfig{
			TaskBudget: &anthropicTaskBudget{
				Type:      "tokens",
				Total:     req.TaskBudget.Total,
				Remaining: req.TaskBudget.Remaining,
			},
		}
	}

	return msgReq
}

// handleSSEEvent maps one Anthropic SSE event into the shared model stream format.
func (c *Client) handleSSEEvent(eventName, data string, contentBlocks map[int]*streamContentBlock, state *streamState, out chan<- model.Event) {
	if data == "" || data == "[DONE]" {
		return
	}

	switch eventName {
	case "message_start":
		var payload messageStartEnvelope
		if err := json.Unmarshal([]byte(data), &payload); err != nil {
			out <- model.Event{
				Type:  model.EventTypeError,
				Error: fmt.Sprintf("parse anthropic message_start: %v", err),
			}
			return
		}
		state.inputTokens = payload.Message.Usage.InputTokens
		state.cacheCreateTokens = payload.Message.Usage.CacheCreationInputTokens
		state.cacheReadTokens = payload.Message.Usage.CacheReadInputTokens
	case "message_delta":
		var payload messageDeltaEnvelope
		if err := json.Unmarshal([]byte(data), &payload); err != nil {
			out <- model.Event{
				Type:  model.EventTypeError,
				Error: fmt.Sprintf("parse anthropic message_delta: %v", err),
			}
			return
		}
		state.outputTokens = payload.Usage.OutputTokens
		if payload.Delta.StopReason != "" {
			state.stopReason = payload.Delta.StopReason
		}
	case "content_block_start":
		var payload contentBlockStartEnvelope
		if err := json.Unmarshal([]byte(data), &payload); err != nil {
			out <- model.Event{
				Type:  model.EventTypeError,
				Error: fmt.Sprintf("parse anthropic content block start: %v", err),
			}
			return
		}
		switch payload.ContentBlock.Type {
		case "tool_use":
			contentBlocks[payload.Index] = &streamContentBlock{
				blockType: payload.ContentBlock.Type,
				toolID:    payload.ContentBlock.ID,
				toolName:  payload.ContentBlock.Name,
			}
		case "thinking":
			contentBlocks[payload.Index] = &streamContentBlock{
				blockType: payload.ContentBlock.Type,
			}
		case "redacted_thinking":
			contentBlocks[payload.Index] = &streamContentBlock{
				blockType: payload.ContentBlock.Type,
				data:      payload.ContentBlock.Data,
			}
		}
	case "content_block_delta":
		var payload contentBlockDeltaEnvelope
		if err := json.Unmarshal([]byte(data), &payload); err != nil {
			out <- model.Event{
				Type:  model.EventTypeError,
				Error: fmt.Sprintf("parse anthropic text delta: %v", err),
			}
			return
		}
		if payload.Delta.Type == "text_delta" && payload.Delta.Text != "" {
			out <- model.Event{
				Type: model.EventTypeTextDelta,
				Text: payload.Delta.Text,
			}
		}
		if payload.Delta.Type == "thinking_delta" {
			block, ok := contentBlocks[payload.Index]
			if !ok || block.blockType != "thinking" {
				out <- model.Event{
					Type:  model.EventTypeError,
					Error: "received thinking delta for unknown content block",
				}
				return
			}
			block.thinking.WriteString(payload.Delta.Thinking)
		}
		if payload.Delta.Type == "signature_delta" {
			block, ok := contentBlocks[payload.Index]
			if !ok || (block.blockType != "thinking" && block.blockType != "connector_text") {
				out <- model.Event{
					Type:  model.EventTypeError,
					Error: "received signature delta for unknown content block",
				}
				return
			}
			block.signature = payload.Delta.Signature
		}
		if payload.Delta.Type == "input_json_delta" {
			block, ok := contentBlocks[payload.Index]
			if !ok || block.blockType != "tool_use" {
				out <- model.Event{
					Type:  model.EventTypeError,
					Error: "received tool input delta for unknown content block",
				}
				return
			}
			block.inputJSON.WriteString(payload.Delta.PartialJSON)
		}
	case "content_block_stop":
		var payload contentBlockStopEnvelope
		if err := json.Unmarshal([]byte(data), &payload); err != nil {
			out <- model.Event{
				Type:  model.EventTypeError,
				Error: fmt.Sprintf("parse anthropic content block stop: %v", err),
			}
			return
		}

		block, ok := contentBlocks[payload.Index]
		if !ok {
			return
		}
		defer delete(contentBlocks, payload.Index)

		switch block.blockType {
		case "tool_use":
			input, err := parseToolUseInput(block.inputJSON.String())
			if err != nil {
				out <- model.Event{
					Type:  model.EventTypeError,
					Error: fmt.Sprintf("parse anthropic tool use input: %v", err),
				}
				return
			}
			out <- model.Event{
				Type: model.EventTypeToolUse,
				ToolUse: &model.ToolUse{
					ID:    block.toolID,
					Name:  block.toolName,
					Input: input,
				},
			}
		case "thinking":
			out <- model.Event{
				Type:      model.EventTypeThinking,
				Thinking:  block.thinking.String(),
				Signature: block.signature,
			}
		case "redacted_thinking":
			out <- model.Event{
				Type:      model.EventTypeThinking,
				Thinking:  block.data,
				Signature: "",
			}
		}
	case "error":
		var payload errorEnvelope
		if err := json.Unmarshal([]byte(data), &payload); err != nil {
			out <- model.Event{
				Type:  model.EventTypeError,
				Error: fmt.Sprintf("parse anthropic error event: %v", err),
			}
			return
		}
		out <- model.Event{
			Type:  model.EventTypeError,
			Error: payload.Error.Message,
		}
	}
}

// parseRetryAfter extracts the retry delay from HTTP response headers.
// It supports both seconds (e.g. "120") and RFC1123 date strings
// (e.g. "Wed, 21 Oct 2025 07:28:00 GMT").  The returned bool is false
// when the header is missing or cannot be parsed.
func parseRetryAfter(header http.Header) (time.Duration, bool) {
	value := strings.TrimSpace(header.Get("retry-after"))
	if value == "" {
		return 0, false
	}

	// Try seconds first (most common from Anthropic).
	if seconds, err := strconv.Atoi(value); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second, true
	}

	// Fall back to RFC1123 date parsing.
	if ts, err := time.Parse(time.RFC1123, value); err == nil {
		wait := time.Until(ts)
		if wait < 0 {
			wait = 0
		}
		return wait, true
	}

	return 0, false
}

// rateLimitHeaders holds the subset of Anthropic rate-limit headers that
// influence retry / back-off decisions.
type rateLimitHeaders struct {
	// Remaining is the number of requests remaining in the current window.
	Remaining int
	// Reset is the unix timestamp (seconds) when the current limit window resets.
	Reset int64
	// RequestLimit is the per-window request limit.
	RequestLimit int
}

// parseRateLimitHeaders extracts Anthropic-specific rate-limit fields from
// the response headers.  Missing or malformed fields are silently ignored.
func parseRateLimitHeaders(header http.Header) rateLimitHeaders {
	return rateLimitHeaders{
		Remaining:    parseHeaderInt(header, "x-ratelimit-remaining"),
		Reset:        parseHeaderInt64(header, "x-ratelimit-reset"),
		RequestLimit: parseHeaderInt(header, "x-ratelimit-limit"),
	}
}

// parseHeaderInt converts an optional integer header into an int, tolerating
// invalid values.
func parseHeaderInt(headers http.Header, key string) int {
	value := strings.TrimSpace(headers.Get(key))
	if value == "" {
		return 0
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return parsed
}

// computeBackoff returns the duration to wait before the next retry attempt.
// It prefers the retry-after header when present; otherwise it falls back to
// the rate-limit reset time.  If neither is available it returns false.
func computeBackoff(header http.Header) (time.Duration, bool) {
	// 1. Explicit retry-after takes highest priority.
	if d, ok := parseRetryAfter(header); ok {
		return d, true
	}

	// 2. Fall back to x-ratelimit-reset (unix seconds).
	rl := parseRateLimitHeaders(header)
	if rl.Reset > 0 {
		wait := time.Until(time.Unix(rl.Reset, 0))
		if wait < 0 {
			wait = 0
		}
		return wait, true
	}

	return 0, false
}

// retryConfig holds the parameters for exponential backoff.
// These match the batch-67 defaults.
type retryConfig struct {
	baseDelay  time.Duration
	maxDelay   time.Duration
	jitterFrac float64
}

// defaultRetryConfig returns the batch-67 default retry parameters.
func defaultRetryConfig() retryConfig {
	return retryConfig{
		baseDelay:  500 * time.Millisecond,
		maxDelay:   30 * time.Second,
		jitterFrac: 0.25,
	}
}

// exponentialBackoff computes the delay for the given attempt using
// baseDelay * 2^attempt capped at maxDelay, plus up to jitterFrac jitter.
func exponentialBackoff(cfg retryConfig, attempt int) time.Duration {
	if cfg.baseDelay <= 0 {
		cfg.baseDelay = 500 * time.Millisecond
	}
	if cfg.maxDelay <= 0 {
		cfg.maxDelay = 30 * time.Second
	}
	if cfg.jitterFrac <= 0 {
		cfg.jitterFrac = 0.25
	}

	exp := time.Duration(float64(cfg.baseDelay) * math.Pow(2, float64(attempt)))
	if exp > cfg.maxDelay {
		exp = cfg.maxDelay
	}

	jitter := time.Duration(rand.Int63n(int64(float64(exp) * cfg.jitterFrac)))
	return exp + jitter
}

// computeRetryDelay returns the wait duration before the next retry attempt.
//
// Rules:
//   - If err is nil or not retryable, returns an error (no wait).
//   - If the error is rate-limit or overloaded, prefers the server's
//     retry-after / rate-limit-reset headers; falls back to exponential
//     backoff if headers are absent.
//   - For other retryable errors, uses exponential backoff.
//
// attempt is zero-based (0 = first retry).
func computeRetryDelay(err *APIError, attempt int) (time.Duration, error) {
	if err == nil || !err.IsRetryable() {
		return 0, fmt.Errorf("error is not retryable")
	}

	// Rate-limit and overloaded errors: prefer server-directed wait time.
	if err.IsRateLimit() || err.IsOverloaded() {
		// 1. retry-after header (from APIError.Headers).
		if d := err.RetryAfter(); d > 0 {
			return d, nil
		}
		// 2. rate-limit reset timestamp.
		if reset := err.RateLimitReset(); reset > 0 {
			wait := time.Until(time.Unix(reset, 0))
			if wait > 0 {
				return wait, nil
			}
		}
		// 3. Fall back to exponential backoff.
		return exponentialBackoff(defaultRetryConfig(), attempt), nil
	}

	// All other retryable errors: exponential backoff.
	return exponentialBackoff(defaultRetryConfig(), attempt), nil
}

// parseToolUseInput decodes the final accumulated tool-use JSON payload.
func parseToolUseInput(raw string) (map[string]any, error) {
	if strings.TrimSpace(raw) == "" {
		return map[string]any{}, nil
	}

	var input map[string]any
	if err := json.Unmarshal([]byte(raw), &input); err != nil {
		return nil, err
	}
	if input == nil {
		return map[string]any{}, nil
	}
	return input, nil
}

// mapMessages converts shared message types into the Anthropic request shape.
// When enablePromptCaching is true, a single cache_control marker is placed
// on the last eligible content block of the last message, matching the TS
// addCacheBreakpoints rule.
func mapMessages(messages []message.Message, enablePromptCaching bool) []anthropicMessage {
	out := make([]anthropicMessage, 0, len(messages))
	for _, msg := range messages {
		item := anthropicMessage{
			Role:    string(msg.Role),
			Content: make([]anthropicContentBlock, 0, len(msg.Content)),
		}
		for _, part := range msg.Content {
			switch part.Type {
			case "text":
				item.Content = append(item.Content, anthropicContentBlock{
					Type: "text",
					Text: part.Text,
				})
			case "tool_use":
				item.Content = append(item.Content, anthropicContentBlock{
					Type:  "tool_use",
					ID:    part.ToolUseID,
					Name:  part.ToolName,
					Input: part.ToolInput,
				})
			case "tool_result":
				item.Content = append(item.Content, anthropicContentBlock{
					Type:      "tool_result",
					ToolUseID: part.ToolUseID,
					Content:   part.Text,
					IsError:   part.IsError,
				})
			case "thinking":
				item.Content = append(item.Content, anthropicContentBlock{
					Type:      "thinking",
					Thinking:  part.Thinking,
					Signature: part.Signature,
				})
			case "redacted_thinking":
				item.Content = append(item.Content, anthropicContentBlock{
					Type: "redacted_thinking",
					Data: part.Data,
				})
			}
		}
		if len(item.Content) == 0 {
			continue
		}
		out = append(out, item)
	}

	// Place a single cache_control marker on the last content block of the
	// last message, skipping thinking/redacted_thinking blocks for assistant
	// messages to match TS assistantMessageToMessageParam behavior.
	if enablePromptCaching && len(out) > 0 {
		lastMsg := &out[len(out)-1]
		if len(lastMsg.Content) > 0 {
			lastIdx := len(lastMsg.Content) - 1
			if lastMsg.Role == string(message.RoleAssistant) {
				for lastIdx >= 0 {
					block := lastMsg.Content[lastIdx]
					if block.Type == "thinking" || block.Type == "redacted_thinking" {
						lastIdx--
					} else {
						break
					}
				}
			}
			if lastIdx >= 0 {
				lastMsg.Content[lastIdx].CacheControl = &cacheControl{Type: "ephemeral"}
			}
		}
	}

	return out
}

// mapTools converts shared tool declarations into the Anthropic request shape.
func mapTools(tools []model.ToolDefinition) []anthropicTool {
	if len(tools) == 0 {
		return nil
	}

	out := make([]anthropicTool, 0, len(tools))
	for _, toolDef := range tools {
		if strings.TrimSpace(toolDef.Name) == "" {
			continue
		}
		out = append(out, anthropicTool{
			Name:        toolDef.Name,
			Description: toolDef.Description,
			InputSchema: toolDef.InputSchema,
		})
	}
	return out
}
