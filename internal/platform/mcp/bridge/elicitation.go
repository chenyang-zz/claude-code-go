package bridge

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sheepzhao/claude-code-go/internal/core/hook"
	"github.com/sheepzhao/claude-code-go/internal/platform/mcp/client"
	runtimehooks "github.com/sheepzhao/claude-code-go/internal/runtime/hooks"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// elicitationHookRunner is the hook execution surface needed by the elicitation bridge.
type elicitationHookRunner interface {
	RunHooksForEvent(ctx context.Context, config hook.HooksConfig, event hook.HookEvent, input any, cwd string) []hook.HookResult
	RunHooksForEventWithQuery(ctx context.Context, config hook.HooksConfig, event hook.HookEvent, input any, cwd string, query runtimehooks.MatchQuery) []hook.HookResult
}

// RegisterElicitationHandlers wires MCP elicitation request and completion handlers onto one client.
func RegisterElicitationHandlers(c *client.Client, serverName string, hookRunner elicitationHookRunner, hooksCfg hook.HooksConfig, disableAllHooks bool) error {
	if c == nil {
		return nil
	}

	if err := c.SetRequestHandler(client.ElicitRequestMethod, func(req client.JSONRPCRequest) (any, error) {
		params, err := decodeElicitRequestParams(req.Params)
		if err != nil {
			return nil, err
		}

		logger.DebugCF("mcp", "received elicitation request", map[string]any{
			"server": serverName,
			"mode":   params.Mode,
		})

		result := client.ElicitResult{Action: "cancel"}
		if !disableAllHooks && hookRunner != nil {
			response, blocking := runElicitationHooks(serverName, hookRunner, hooksCfg, params)
			if response != nil {
				result = *response
			}
			if blocking {
				result.Action = "decline"
			}
		}
		if result.Action == "" {
			result.Action = "cancel"
		}
		if result.Action != "accept" {
			result.Content = nil
		}

		if !disableAllHooks && hookRunner != nil {
			if override, err := runElicitationResultHooks(serverName, hookRunner, hooksCfg, params, result); err != nil {
				return nil, err
			} else if override != nil {
				result = *override
			}
		}
		if result.Action == "" {
			result.Action = "cancel"
		}
		if result.Action != "accept" {
			result.Content = nil
		}
		return result, nil
	}); err != nil {
		return err
	}

	return c.SetNotificationHandler(client.ElicitationCompleteNotificationMethod, func(notification client.JSONRPCNotification) {
		if disableAllHooks || hookRunner == nil {
			return
		}
		var payload client.ElicitationCompleteNotification
		if err := json.Unmarshal(notification.Params, &payload); err != nil {
			logger.WarnCF("mcp", "failed to decode elicitation completion notification", map[string]any{
				"server": serverName,
				"error":  err.Error(),
			})
			return
		}

		input := hook.NotificationHookInput{
			BaseHookInput: hook.BaseHookInput{},
			HookEventName: string(hook.EventNotification),
			Message:       fmt.Sprintf(`MCP server "%s" confirmed elicitation %s complete`, serverName, payload.ElicitationID),
			Title:         "MCP Elicitation Complete",
			NotificationType: "elicitation_complete",
		}
		go hookRunner.RunHooksForEvent(context.Background(), hooksCfg, hook.EventNotification, input, "")
	})
}

// decodeElicitRequestParams decodes the JSON-RPC params payload into an elicitation request model.
func decodeElicitRequestParams(raw json.RawMessage) (client.ElicitRequestParams, error) {
	var params client.ElicitRequestParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return client.ElicitRequestParams{}, fmt.Errorf("mcp elicitation: decode params: %w", err)
	}
	if params.Mode == "" {
		params.Mode = "form"
	}
	return params, nil
}

// runElicitationHooks executes server-specific elicitation hooks and converts the first response into an MCP result.
func runElicitationHooks(serverName string, hookRunner elicitationHookRunner, hooksCfg hook.HooksConfig, params client.ElicitRequestParams) (*client.ElicitResult, bool) {
	input := hook.ElicitationHookInput{
		HookEventName:   string(hook.EventElicitation),
		MCPServerName:   serverName,
		Message:         params.Message,
		Mode:            params.Mode,
		URL:             params.URL,
		ElicitationID:   params.ElicitationID,
		RequestedSchema: params.RequestedSchema,
	}

	results := hookRunner.RunHooksForEventWithQuery(context.Background(), hooksCfg, hook.EventElicitation, input, "", runtimehooks.MatchQuery{Matcher: serverName})
	return convertElicitationHookResults(results, string(hook.EventElicitation))
}

// runElicitationResultHooks executes server-specific elicitation result hooks and converts the first response into an MCP result.
func runElicitationResultHooks(serverName string, hookRunner elicitationHookRunner, hooksCfg hook.HooksConfig, params client.ElicitRequestParams, result client.ElicitResult) (*client.ElicitResult, error) {
	input := hook.ElicitationResultHookInput{
		HookEventName: string(hook.EventElicitationResult),
		MCPServerName: serverName,
		ElicitationID: params.ElicitationID,
		Mode:          params.Mode,
		Action:        result.Action,
		Content:       result.Content,
	}

	results := hookRunner.RunHooksForEventWithQuery(context.Background(), hooksCfg, hook.EventElicitationResult, input, "", runtimehooks.MatchQuery{Matcher: serverName})
	override, _ := convertElicitationHookResults(results, string(hook.EventElicitationResult))
	if override == nil {
		return nil, nil
	}
	return override, nil
}

// convertElicitationHookResults maps hook command output into an elicitation result.
func convertElicitationHookResults(results []hook.HookResult, expectedEventName string) (*client.ElicitResult, bool) {
	var final *client.ElicitResult
	blocking := false

	for _, result := range results {
		if result.ParsedOutput == nil {
			continue
		}
		if result.IsBlocking() {
			blocking = true
			final = &client.ElicitResult{Action: "decline"}
			continue
		}
		if result.ParsedOutput.Decision != nil && *result.ParsedOutput.Decision == hook.DecisionBlock {
			blocking = true
			final = &client.ElicitResult{Action: "decline"}
			continue
		}
		if len(result.ParsedOutput.HookSpecificOutput) == 0 {
			continue
		}

		var specific struct {
			HookEventName string         `json:"hookEventName"`
			Action        string         `json:"action,omitempty"`
			Content       map[string]any `json:"content,omitempty"`
		}
		if err := json.Unmarshal(result.ParsedOutput.HookSpecificOutput, &specific); err != nil {
			continue
		}
		if specific.HookEventName != expectedEventName || specific.Action == "" {
			continue
		}

		final = &client.ElicitResult{
			Action:  specific.Action,
			Content: specific.Content,
		}
		if specific.Action == "decline" {
			blocking = true
		}
	}
	return final, blocking
}
