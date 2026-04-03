package tool

// UseContext carries the minimal host data shared with a single tool invocation.
type UseContext struct {
	// WorkingDir is the caller-visible working directory used to resolve relative paths.
	WorkingDir string
	// Invoker identifies the upstream component that initiated the tool call.
	Invoker    string
}

// Call is the normalized input envelope passed from the runtime into a tool.
type Call struct {
	// ID is the per-call identifier used for tracing and correlation.
	ID string
	// Name is the registered tool name the executor should dispatch to.
	Name string
	// Input stores the decoded tool arguments as a generic key/value payload.
	Input map[string]any
	// Source records where the call originated from, such as a CLI or test harness.
	Source string
	// Context carries the host execution context shared with the tool implementation.
	Context UseContext
}
