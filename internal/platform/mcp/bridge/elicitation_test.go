package bridge

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/hook"
	"github.com/sheepzhao/claude-code-go/internal/platform/mcp/client"
	runtimehooks "github.com/sheepzhao/claude-code-go/internal/runtime/hooks"
)

type elicitationMockTransport struct {
	requestHandlers      map[string]client.RequestHandler
	notificationHandlers map[string]client.NotificationHandler
}

func (m *elicitationMockTransport) Send(ctx context.Context, req client.JSONRPCRequest) (*client.JSONRPCResponse, error) {
	return nil, nil
}

func (m *elicitationMockTransport) SetNotificationHandler(method string, handler client.NotificationHandler) {
	if m.notificationHandlers == nil {
		m.notificationHandlers = make(map[string]client.NotificationHandler)
	}
	if handler == nil {
		delete(m.notificationHandlers, method)
		return
	}
	m.notificationHandlers[method] = handler
}

func (m *elicitationMockTransport) SetRequestHandler(method string, handler client.RequestHandler) {
	if m.requestHandlers == nil {
		m.requestHandlers = make(map[string]client.RequestHandler)
	}
	if handler == nil {
		delete(m.requestHandlers, method)
		return
	}
	m.requestHandlers[method] = handler
}

func (m *elicitationMockTransport) Close() error { return nil }

type elicitationMockRunner struct {
	results         []hook.HookResult
	lastEvent       hook.HookEvent
	lastQuery       runtimehooks.MatchQuery
	lastInput       any
	eventCalled     chan struct{}
	withQueryCalled chan struct{}
}

func (r *elicitationMockRunner) RunHooksForEvent(ctx context.Context, config hook.HooksConfig, event hook.HookEvent, input any, cwd string) []hook.HookResult {
	r.lastEvent = event
	r.lastInput = input
	if r.eventCalled != nil {
		select {
		case r.eventCalled <- struct{}{}:
		default:
		}
	}
	return r.results
}

func (r *elicitationMockRunner) RunHooksForEventWithQuery(ctx context.Context, config hook.HooksConfig, event hook.HookEvent, input any, cwd string, query runtimehooks.MatchQuery) []hook.HookResult {
	r.lastEvent = event
	r.lastInput = input
	r.lastQuery = query
	if r.withQueryCalled != nil {
		select {
		case r.withQueryCalled <- struct{}{}:
		default:
		}
	}
	return r.results
}

func TestRegisterElicitationHandlersReturnsCancelByDefault(t *testing.T) {
	transport := &elicitationMockTransport{}
	c := client.NewClient(transport)

	runner := &elicitationMockRunner{}
	if err := RegisterElicitationHandlers(c, "demo", runner, nil, false); err != nil {
		t.Fatalf("RegisterElicitationHandlers: %v", err)
	}

	handler := transport.requestHandlers[client.ElicitRequestMethod]
	if handler == nil {
		t.Fatal("request handler not registered")
	}

	result, err := handler(client.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      "req-1",
		Method:  client.ElicitRequestMethod,
		Params:  json.RawMessage(`{"mode":"form","message":"Need input"}`),
	})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}

	elicitResult, ok := result.(client.ElicitResult)
	if !ok {
		t.Fatalf("result type = %T, want client.ElicitResult", result)
	}
	if elicitResult.Action != "cancel" {
		t.Fatalf("action = %q, want cancel", elicitResult.Action)
	}
}

func TestRegisterElicitationHandlersUsesHookResponse(t *testing.T) {
	transport := &elicitationMockTransport{}
	c := client.NewClient(transport)

	runner := &elicitationMockRunner{
		results: []hook.HookResult{
			{
				ParsedOutput: hook.ParseHookOutput(`{"hookSpecificOutput":{"hookEventName":"Elicitation","action":"accept","content":{"token":"abc"}}}`),
			},
		},
	}
	if err := RegisterElicitationHandlers(c, "demo-server", runner, nil, false); err != nil {
		t.Fatalf("RegisterElicitationHandlers: %v", err)
	}

	handler := transport.requestHandlers[client.ElicitRequestMethod]
	if handler == nil {
		t.Fatal("request handler not registered")
	}

	result, err := handler(client.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      "req-2",
		Method:  client.ElicitRequestMethod,
		Params:  json.RawMessage(`{"mode":"form","message":"Need input","requestedSchema":{"type":"object"}}`),
	})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}

	elicitResult, ok := result.(client.ElicitResult)
	if !ok {
		t.Fatalf("result type = %T, want client.ElicitResult", result)
	}
	if elicitResult.Action != "accept" {
		t.Fatalf("action = %q, want accept", elicitResult.Action)
	}
	if elicitResult.Content["token"] != "abc" {
		t.Fatalf("content = %#v, want token=abc", elicitResult.Content)
	}
	if runner.lastQuery.Matcher != "demo-server" {
		t.Fatalf("matcher = %q, want demo-server", runner.lastQuery.Matcher)
	}
}

func TestRegisterElicitationHandlersFiresCompletionNotificationHooks(t *testing.T) {
	transport := &elicitationMockTransport{}
	c := client.NewClient(transport)

	runner := &elicitationMockRunner{
		eventCalled: make(chan struct{}, 1),
	}
	if err := RegisterElicitationHandlers(c, "demo-server", runner, nil, false); err != nil {
		t.Fatalf("RegisterElicitationHandlers: %v", err)
	}

	handler := transport.notificationHandlers[client.ElicitationCompleteNotificationMethod]
	if handler == nil {
		t.Fatal("notification handler not registered")
	}

	handler(client.JSONRPCNotification{
		JSONRPC: "2.0",
		Method:  client.ElicitationCompleteNotificationMethod,
		Params:  json.RawMessage(`{"elicitationId":"elic-1"}`),
	})

	select {
	case <-runner.eventCalled:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for notification hook")
	}

	switch input := runner.lastInput.(type) {
	case hook.NotificationHookInput:
		if !strings.Contains(input.Message, "elic-1") {
			t.Fatalf("notification message = %q, want elicitation id", input.Message)
		}
	default:
		t.Fatalf("input type = %T, want hook.NotificationHookInput", runner.lastInput)
	}
}
