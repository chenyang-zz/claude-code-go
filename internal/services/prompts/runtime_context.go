package prompts

import "context"

// RuntimeContext carries session-scoped prompt inputs that cannot be derived
// from the static section types alone.
type RuntimeContext struct {
	// EnabledToolNames lists the tool names available in the current runtime.
	EnabledToolNames map[string]struct{}
	// WorkingDir stores the active workspace directory for session-scoped prompt data.
	WorkingDir string
	// SessionID stores the current logical session identifier for session-scoped prompt data.
	SessionID string
}

type runtimeContextKey struct{}

// WithRuntimeContext attaches runtime prompt inputs to the context passed into
// PromptBuilder.Build.
func WithRuntimeContext(ctx context.Context, data RuntimeContext) context.Context {
	return context.WithValue(ctx, runtimeContextKey{}, data)
}

// RuntimeContextFromContext extracts runtime prompt inputs from ctx.
func RuntimeContextFromContext(ctx context.Context) (RuntimeContext, bool) {
	data, ok := ctx.Value(runtimeContextKey{}).(RuntimeContext)
	return data, ok
}

// HasTool reports whether a tool is available in the current runtime context.
func (c RuntimeContext) HasTool(name string) bool {
	if len(c.EnabledToolNames) == 0 {
		return false
	}
	_, ok := c.EnabledToolNames[name]
	return ok
}

// HasAnyTool reports whether any of the named tools are available in the current runtime context.
func (c RuntimeContext) HasAnyTool(names ...string) bool {
	for _, name := range names {
		if c.HasTool(name) {
			return true
		}
	}
	return false
}
