package remote_trigger

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/tool"
	"github.com/sheepzhao/claude-code-go/internal/services/policylimits"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const Name = "RemoteTrigger"

// Tool provides access to the claude.ai remote-trigger CRUD API.
// It wraps the /v1/code/triggers endpoints with in-process OAuth token handling
// so that credentials are never exposed to the shell.
type Tool struct {
	httpClient *http.Client
}

// NewTool creates a Tool with a default 20-second HTTP timeout.
func NewTool() *Tool {
	return &Tool{
		httpClient: &http.Client{Timeout: 20 * time.Second},
	}
}

// Name returns the stable tool identifier used during registration and dispatch.
func (t *Tool) Name() string {
	return Name
}

// Description returns a short human-readable summary for the tool.
func (t *Tool) Description() string {
	return "Manage scheduled remote Claude Code agents (triggers) via the claude.ai CCR API. Auth is handled in-process — the token never reaches the shell."
}

// IsReadOnly returns false because create, update, and run are mutating operations.
func (t *Tool) IsReadOnly() bool {
	return false
}

// IsConcurrencySafe reports that multiple invocations can run in parallel safely.
func (t *Tool) IsConcurrencySafe() bool {
	return true
}

// InputSchema declares the accepted input fields for the remote trigger tool.
func (t *Tool) InputSchema() tool.InputSchema {
	return tool.InputSchema{
		Properties: map[string]tool.FieldSchema{
			"action": {
				Type:        tool.ValueKindString,
				Description: "The action to perform: list, get, create, update, or run.",
				Required:    true,
			},
			"trigger_id": {
				Type:        tool.ValueKindString,
				Description: "Required for get, update, and run. Must match ^[\\w-]+$.",
				Required:    false,
			},
			"body": {
				Type:        tool.ValueKindObject,
				Description: "JSON body for create and update.",
				Required:    false,
			},
		},
	}
}

// remoteTriggerInput is the typed input decoded from the raw tool call payload.
type remoteTriggerInput struct {
	Action    string                 `json:"action"`
	TriggerID string                 `json:"trigger_id,omitempty"`
	Body      map[string]any `json:"body,omitempty"`
}

// remoteTriggerOutput is the structured result stored in tool call metadata.
type remoteTriggerOutput struct {
	Status int    `json:"status"`
	JSON   string `json:"json"`
}

// triggerIDPattern matches the TS regex /^[\w-]+$/ for trigger identifiers.
var triggerIDPattern = regexp.MustCompile(`^[\w-]+$`)

// Invoke executes the remote trigger CRUD action against the claude.ai API.
func (t *Tool) Invoke(ctx context.Context, call tool.Call) (tool.Result, error) {
	input, err := tool.DecodeInput[remoteTriggerInput](t.InputSchema(), call.Input)
	if err != nil {
		return tool.Result{Error: err.Error()}, nil
	}

	if allowed, reason := policylimits.IsAllowed(policylimits.ActionAllowRemoteSessions); !allowed {
		return tool.Result{Error: reason}, nil
	}

	// Validate action enum membership.
	switch input.Action {
	case "list", "get", "create", "update", "run":
	default:
		return tool.Result{Error: fmt.Sprintf("invalid action %q: must be one of list, get, create, update, run", input.Action)}, nil
	}

	// Enforce action-specific required fields.
	if (input.Action == "get" || input.Action == "update" || input.Action == "run") && strings.TrimSpace(input.TriggerID) == "" {
		return tool.Result{Error: fmt.Sprintf("%s requires trigger_id", input.Action)}, nil
	}
	if input.TriggerID != "" && !triggerIDPattern.MatchString(input.TriggerID) {
		return tool.Result{Error: fmt.Sprintf("invalid trigger_id %q: must match ^[\\w-]+$", input.TriggerID)}, nil
	}
	if (input.Action == "create" || input.Action == "update") && input.Body == nil {
		return tool.Result{Error: fmt.Sprintf("%s requires body", input.Action)}, nil
	}

	// Resolve OAuth access token from environment.
	accessToken := strings.TrimSpace(os.Getenv("CLAUDE_CODE_OAUTH_TOKEN"))
	if accessToken == "" {
		return tool.Result{Error: "Not authenticated with a claude.ai account. Set CLAUDE_CODE_OAUTH_TOKEN or run /login and try again."}, nil
	}

	// Resolve organization UUID from environment.
	orgUUID := strings.TrimSpace(os.Getenv("CLAUDE_CODE_ORGANIZATION_UUID"))
	if orgUUID == "" {
		return tool.Result{Error: "Unable to resolve organization UUID. Set CLAUDE_CODE_ORGANIZATION_UUID."}, nil
	}

	// Build the HTTP request.
	method, urlPath, bodyBytes, err := buildRequest(input)
	if err != nil {
		return tool.Result{Error: err.Error()}, nil
	}

	baseURL := strings.TrimSpace(os.Getenv("CLAUDE_CODE_API_BASE_URL"))
	if baseURL == "" {
		baseURL = "https://api.claude.ai"
	}
	baseURL = strings.TrimRight(baseURL, "/")

	req, err := http.NewRequestWithContext(ctx, method, baseURL+urlPath, bytes.NewReader(bodyBytes))
	if err != nil {
		return tool.Result{Error: fmt.Sprintf("failed to create request: %s", err.Error())}, nil
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("anthropic-beta", "ccr-triggers-2026-01-30")
	req.Header.Set("x-organization-uuid", orgUUID)

	logger.DebugCF("remote_trigger", "sending remote trigger API request", map[string]any{
		"action":     input.Action,
		"method":     method,
		"url":        baseURL + urlPath,
		"body_bytes": len(bodyBytes),
	})

	res, err := t.httpClient.Do(req)
	if err != nil {
		logger.DebugCF("remote_trigger", "HTTP request failed", map[string]any{
			"action": input.Action,
			"error":  err.Error(),
		})
		return tool.Result{Error: fmt.Sprintf("HTTP request failed: %s", err.Error())}, nil
	}
	defer res.Body.Close()

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return tool.Result{Error: fmt.Sprintf("failed to read response: %s", err.Error())}, nil
	}

	logger.DebugCF("remote_trigger", "remote trigger API call completed", map[string]any{
		"action":      input.Action,
		"status_code": res.StatusCode,
		"body_length": len(resBody),
	})

	return tool.Result{
		Output: fmt.Sprintf("HTTP %d\n%s", res.StatusCode, string(resBody)),
		Meta: map[string]any{
			"data": remoteTriggerOutput{
				Status: res.StatusCode,
				JSON:   string(resBody),
			},
		},
	}, nil
}

// buildRequest translates the decoded tool input into an HTTP method, URL path suffix, and optional JSON body.
func buildRequest(input remoteTriggerInput) (method, urlPath string, body []byte, err error) {
	switch input.Action {
	case "list":
		return http.MethodGet, "/v1/code/triggers", nil, nil
	case "get":
		return http.MethodGet, "/v1/code/triggers/" + input.TriggerID, nil, nil
	case "create":
		body, err = json.Marshal(input.Body)
		return http.MethodPost, "/v1/code/triggers", body, err
	case "update":
		body, err = json.Marshal(input.Body)
		return http.MethodPost, "/v1/code/triggers/" + input.TriggerID, body, err
	case "run":
		return http.MethodPost, "/v1/code/triggers/" + input.TriggerID + "/run", []byte("{}"), nil
	default:
		return "", "", nil, fmt.Errorf("unknown action %q", input.Action)
	}
}
