package openai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"

	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
	"github.com/sheepzhao/claude-code-go/internal/core/message"
	"github.com/sheepzhao/claude-code-go/internal/core/model"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const (
	// defaultBaseURL stores the canonical OpenAI API host root.
	defaultBaseURL = "https://api.openai.com"
	// defaultChatCompletionsPath stores the minimum OpenAI-compatible chat completions endpoint.
	defaultChatCompletionsPath = "/v1/chat/completions"
	// glmDefaultBaseURL stores the GLM OpenAI-compatible API host root.
	glmDefaultBaseURL = "https://open.bigmodel.cn/api/paas"
	// glmChatCompletionsPath stores the GLM chat completions endpoint.
	glmChatCompletionsPath = "/v4/chat/completions"
)

// Config carries the minimum OpenAI-compatible client configuration used by the migrated runtime.
type Config struct {
	// Provider selects whether the generic OpenAI-compatible or GLM defaults should be used.
	Provider string
	// APIKey carries the provider credential.
	APIKey string
	// BaseURL optionally overrides the provider API host.
	BaseURL string
	// HTTPClient allows tests to inject a local transport.
	HTTPClient *http.Client
}

// Client implements the minimum OpenAI-compatible streaming client used by the runtime engine.
type Client struct {
	// provider stores the normalized runtime provider.
	provider string
	// apiKey stores the request credential.
	apiKey string
	// baseURL stores the API host root.
	baseURL string
	// chatPath stores the provider-specific chat completions path.
	chatPath string
	// httpClient performs HTTP requests.
	httpClient *http.Client
}

// chatCompletionsRequest stores the minimal OpenAI-compatible request payload used by the runtime.
type chatCompletionsRequest struct {
	Model               string             `json:"model"`
	Messages            []chatMessage      `json:"messages"`
	Stream              bool               `json:"stream"`
	StreamOptions       *streamOptionsBody `json:"stream_options,omitempty"`
	MaxCompletionTokens int                `json:"max_completion_tokens,omitempty"`
	MaxTokens           int                `json:"max_tokens,omitempty"`
	Tools               []toolEnvelope     `json:"tools,omitempty"`
	ToolChoice          string             `json:"tool_choice,omitempty"`
}

type streamOptionsBody struct {
	IncludeUsage bool `json:"include_usage"`
}

// chatMessage stores one OpenAI-compatible conversation message payload.
type chatMessage struct {
	Role       string         `json:"role"`
	Content    string         `json:"content,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
	ToolCalls  []toolCallBody `json:"tool_calls,omitempty"`
}

// toolEnvelope stores one function tool definition for OpenAI-compatible requests.
type toolEnvelope struct {
	Type     string       `json:"type"`
	Function toolSpecBody `json:"function"`
}

// toolSpecBody stores the function definition payload attached to one tool declaration.
type toolSpecBody struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters"`
}

// toolCallBody stores one assistant-side tool call attached to an OpenAI-compatible assistant message.
type toolCallBody struct {
	ID       string           `json:"id,omitempty"`
	Type     string           `json:"type"`
	Function toolFunctionBody `json:"function"`
}

// toolFunctionBody stores the function name and serialized arguments for one tool call.
type toolFunctionBody struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

// streamEnvelope stores the minimal SSE payload shape emitted by OpenAI-compatible chat completions.
type streamEnvelope struct {
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
	Choices []struct {
		Delta struct {
			Content   string                `json:"content"`
			ToolCalls []streamToolCallDelta `json:"tool_calls"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage,omitempty"`
}

// openAIStopReasonMap maps OpenAI finish_reason values to model stop reasons.
var openAIStopReasonMap = map[string]model.StopReason{
	"stop":           model.StopReasonEndTurn,
	"length":         model.StopReasonMaxTokens,
	"tool_calls":     model.StopReasonToolUse,
	"content_filter": model.StopReasonEndTurn,
}

// openaiStreamState accumulates metadata across one OpenAI-compatible stream.
type openaiStreamState struct {
	finishReason     string
	promptTokens     int
	completionTokens int
}

// streamToolCallDelta stores one partial tool-call delta inside an OpenAI-compatible SSE chunk.
type streamToolCallDelta struct {
	Index    int    `json:"index"`
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// streamToolCall stores one accumulated tool call until its serialized JSON arguments are complete.
type streamToolCall struct {
	// ID stores the provider-generated tool call identifier.
	ID string
	// Name stores the function name selected by the provider.
	Name string
	// Arguments stores the concatenated JSON argument fragments.
	Arguments strings.Builder
}

// NewClient builds a minimal OpenAI-compatible streaming client.
func NewClient(cfg Config) *Client {
	provider := coreconfig.NormalizeProvider(cfg.Provider)
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	return &Client{
		provider:   provider,
		apiKey:     cfg.APIKey,
		baseURL:    strings.TrimRight(resolveBaseURL(provider, cfg.BaseURL), "/"),
		chatPath:   resolveChatCompletionsPath(provider),
		httpClient: httpClient,
	}
}

// Stream opens one OpenAI-compatible streaming request and converts SSE payloads into model events.
func (c *Client) Stream(ctx context.Context, req model.Request) (model.Stream, error) {
	if strings.TrimSpace(c.apiKey) == "" {
		return nil, fmt.Errorf("missing API key for provider %s", c.provider)
	}

	body, err := json.Marshal(chatCompletionsRequest{
		Model:               req.Model,
		Messages:            mapMessages(req.System, req.Messages),
		Stream:              true,
		StreamOptions:       c.streamUsageOption(),
		MaxCompletionTokens: c.maxCompletionTokens(req),
		MaxTokens:           c.maxTokens(req),
		Tools:               mapTools(req.Tools),
		ToolChoice:          renderToolChoice(req.Tools),
	})
	if err != nil {
		return nil, fmt.Errorf("marshal openai-compatible request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+c.chatPath, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build openai-compatible request: %w", err)
	}
	httpReq.Header.Set("content-type", "application/json")
	httpReq.Header.Set("accept", "text/event-stream")
	httpReq.Header.Set("authorization", "Bearer "+c.apiKey)

	logger.DebugCF("openai_client", "starting openai-compatible stream", map[string]any{
		"provider":      c.provider,
		"model":         req.Model,
		"message_count": len(req.Messages),
		"base_url":      c.baseURL,
		"chat_path":     c.chatPath,
	})

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("execute openai-compatible request: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		payload, _ := io.ReadAll(resp.Body)
		return nil, ParseAPIError(resp, payload)
	}

	out := make(chan model.Event)
	go func() {
		defer close(out)
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

		var dataLines []string
		toolCalls := make(map[int]*streamToolCall)
		toolOrder := make([]int, 0)
		var state openaiStreamState

		flush := func() {
			if len(dataLines) == 0 {
				return
			}
			if c.handleChunk(strings.Join(dataLines, "\n"), toolCalls, &toolOrder, &state, out) {
				dataLines = dataLines[:0]
				return
			}
			dataLines = dataLines[:0]
		}

		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				flush()
				continue
			}
			if strings.HasPrefix(line, "data:") {
				dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
			}
		}
		flush()

		if err := scanner.Err(); err != nil {
			out <- model.Event{
				Type:  model.EventTypeError,
				Error: fmt.Sprintf("read openai-compatible stream: %v", err),
			}
			return
		}

		c.emitToolUses(toolCalls, toolOrder, out)

		doneEvent := model.Event{Type: model.EventTypeDone}
		if sr, ok := openAIStopReasonMap[state.finishReason]; ok {
			doneEvent.StopReason = sr
		}
		if state.promptTokens > 0 || state.completionTokens > 0 {
			doneEvent.Usage = &model.Usage{
				InputTokens:  state.promptTokens,
				OutputTokens: state.completionTokens,
			}
		}
		out <- doneEvent
	}()

	return out, nil
}

// handleChunk maps one OpenAI-compatible SSE chunk into shared model events and returns whether the chunk ended the stream.
func (c *Client) handleChunk(data string, toolCalls map[int]*streamToolCall, toolOrder *[]int, state *openaiStreamState, out chan<- model.Event) bool {
	if data == "" {
		return false
	}
	if data == "[DONE]" {
		c.emitToolUses(toolCalls, *toolOrder, out)
		return true
	}

	var payload streamEnvelope
	if err := json.Unmarshal([]byte(data), &payload); err != nil {
		out <- model.Event{
			Type:  model.EventTypeError,
			Error: fmt.Sprintf("parse openai-compatible stream chunk: %v", err),
		}
		return false
	}
	if payload.Error != nil && payload.Error.Message != "" {
		out <- model.Event{
			Type:  model.EventTypeError,
			Error: payload.Error.Message,
		}
		return false
	}

	// Capture usage when the provider includes it (typically the last chunk).
	if payload.Usage != nil {
		state.promptTokens = payload.Usage.PromptTokens
		state.completionTokens = payload.Usage.CompletionTokens
	}

	for _, choice := range payload.Choices {
		if choice.Delta.Content != "" {
			out <- model.Event{
				Type: model.EventTypeTextDelta,
				Text: choice.Delta.Content,
			}
		}
		for _, delta := range choice.Delta.ToolCalls {
			call, ok := toolCalls[delta.Index]
			if !ok {
				call = &streamToolCall{}
				toolCalls[delta.Index] = call
				*toolOrder = append(*toolOrder, delta.Index)
			}
			if delta.ID != "" {
				call.ID = delta.ID
			}
			if delta.Function.Name != "" {
				call.Name = delta.Function.Name
			}
			if delta.Function.Arguments != "" {
				call.Arguments.WriteString(delta.Function.Arguments)
			}
		}
		if choice.FinishReason != "" {
			state.finishReason = choice.FinishReason
		}
		if choice.FinishReason == "tool_calls" {
			c.emitToolUses(toolCalls, *toolOrder, out)
		}
	}

	return false
}

// emitToolUses flushes completed tool calls into shared tool-use stream events in the original provider order.
func (c *Client) emitToolUses(toolCalls map[int]*streamToolCall, order []int, out chan<- model.Event) {
	if len(toolCalls) == 0 {
		return
	}

	sorted := append([]int(nil), order...)
	sort.Ints(sorted)
	for _, index := range sorted {
		call, ok := toolCalls[index]
		if !ok || call == nil {
			continue
		}

		input := make(map[string]any)
		arguments := strings.TrimSpace(call.Arguments.String())
		if arguments != "" {
			if err := json.Unmarshal([]byte(arguments), &input); err != nil {
				out <- model.Event{
					Type:  model.EventTypeError,
					Error: fmt.Sprintf("parse openai-compatible tool arguments: %v", err),
				}
				continue
			}
		}

		out <- model.Event{
			Type: model.EventTypeToolUse,
			ToolUse: &model.ToolUse{
				ID:    call.ID,
				Name:  call.Name,
				Input: input,
			},
		}
		delete(toolCalls, index)
	}
}

// mapMessages converts one normalized conversation into OpenAI-compatible chat-completions messages.
func mapMessages(system string, history []message.Message) []chatMessage {
	out := make([]chatMessage, 0, len(history)+1)
	if strings.TrimSpace(system) != "" {
		out = append(out, chatMessage{
			Role:    string(message.RoleSystem),
			Content: system,
		})
	}

	for _, item := range history {
		switch item.Role {
		case message.RoleUser:
			text := collectText(item.Content)
			if text != "" {
				out = append(out, chatMessage{
					Role:    string(message.RoleUser),
					Content: text,
				})
			}
			for _, part := range item.Content {
				if part.Type != "tool_result" {
					continue
				}
				out = append(out, chatMessage{
					Role:       string(message.RoleTool),
					ToolCallID: part.ToolUseID,
					Content:    part.Text,
				})
			}
		case message.RoleAssistant:
			assistant := chatMessage{
				Role:    string(message.RoleAssistant),
				Content: collectText(item.Content),
			}
			for _, part := range item.Content {
				if part.Type != "tool_use" {
					continue
				}
				arguments, _ := json.Marshal(part.ToolInput)
				assistant.ToolCalls = append(assistant.ToolCalls, toolCallBody{
					ID:   part.ToolUseID,
					Type: "function",
					Function: toolFunctionBody{
						Name:      part.ToolName,
						Arguments: string(arguments),
					},
				})
			}
			if assistant.Content != "" || len(assistant.ToolCalls) > 0 {
				out = append(out, assistant)
			}
		}
	}

	return out
}

// mapTools converts one normalized tool list into OpenAI-compatible function declarations.
func mapTools(tools []model.ToolDefinition) []toolEnvelope {
	if len(tools) == 0 {
		return nil
	}

	out := make([]toolEnvelope, 0, len(tools))
	for _, item := range tools {
		out = append(out, toolEnvelope{
			Type: "function",
			Function: toolSpecBody{
				Name:        item.Name,
				Description: item.Description,
				Parameters:  item.InputSchema,
			},
		})
	}
	return out
}

// renderToolChoice enables automatic function selection when tools are attached to the request.
func renderToolChoice(tools []model.ToolDefinition) string {
	if len(tools) == 0 {
		return ""
	}
	return "auto"
}

// collectText joins all text parts in one normalized message into a stable plain-text payload.
func collectText(parts []message.ContentPart) string {
	chunks := make([]string, 0, len(parts))
	for _, part := range parts {
		if part.Type != "text" || part.Text == "" {
			continue
		}
		chunks = append(chunks, part.Text)
	}
	return strings.Join(chunks, "")
}

// resolveBaseURL selects the provider default host root when the caller does not override it.
func resolveBaseURL(provider, override string) string {
	if strings.TrimSpace(override) != "" {
		return override
	}
	if coreconfig.NormalizeProvider(provider) == coreconfig.ProviderGLM {
		return glmDefaultBaseURL
	}
	return defaultBaseURL
}

// resolveChatCompletionsPath selects the provider-specific chat completions path.
func resolveChatCompletionsPath(provider string) string {
	if coreconfig.NormalizeProvider(provider) == coreconfig.ProviderGLM {
		return glmChatCompletionsPath
	}
	return defaultChatCompletionsPath
}

// streamUsageOption returns stream_options with include_usage only when targeting the standard OpenAI API.
// Other OpenAI-compatible providers may not support this parameter.
// isOfficialOpenAI reports whether the client targets the official OpenAI API.
// It matches when the base URL is the canonical api.openai.com host.
func (c *Client) isOfficialOpenAI() bool {
	return c.baseURL == defaultBaseURL
}

func (c *Client) streamUsageOption() *streamOptionsBody {
	// GLM does not support stream_options; skip for that provider.
	if coreconfig.NormalizeProvider(c.provider) == coreconfig.ProviderGLM {
		return nil
	}
	return &streamOptionsBody{IncludeUsage: true}
}

func (c *Client) maxCompletionTokens(req model.Request) int {
	// Official OpenAI API prefers max_completion_tokens (introduced 2024-12).
	if c.isOfficialOpenAI() && req.MaxOutputTokens > 0 {
		return req.MaxOutputTokens
	}
	return 0
}

func (c *Client) maxTokens(req model.Request) int {
	// Non-offinal OpenAI-compatible providers (including GLM) typically use
	// the legacy max_tokens field.
	if !c.isOfficialOpenAI() && req.MaxOutputTokens > 0 {
		return req.MaxOutputTokens
	}
	return 0
}
