package prompts

import (
	"context"
	"testing"
)

// TestRuntimeContextFromContext verifies runtime prompt inputs can be attached
// to and recovered from a context.Context.
func TestRuntimeContextFromContext(t *testing.T) {
	ctx := WithRuntimeContext(context.Background(), RuntimeContext{
		EnabledToolNames: map[string]struct{}{
			"Agent": {},
			"Read":  {},
		},
	})

	data, ok := RuntimeContextFromContext(ctx)
	if !ok {
		t.Fatal("RuntimeContextFromContext() returned ok=false, want true")
	}
	if !data.HasTool("Agent") {
		t.Fatal("RuntimeContext.HasTool(Agent) = false, want true")
	}
	if data.HasTool("Write") {
		t.Fatal("RuntimeContext.HasTool(Write) = true, want false")
	}
}
