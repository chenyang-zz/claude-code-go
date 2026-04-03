package tool

// Result is the normalized tool response returned to the runtime.
type Result struct {
	// Output contains the primary success payload rendered back to the caller.
	Output string
	// Error carries a tool-level error message when the tool completed with a handled failure.
	Error  string
	// Meta stores auxiliary structured data needed by the caller or tests.
	Meta   map[string]any
}
