package tool

import "time"

// ReadState captures the minimal read snapshot needed to guard later writes.
type ReadState struct {
	// ReadAt records when the file content was last observed by a read tool.
	ReadAt time.Time
	// IsPartial reports whether the recorded read only covered part of the file.
	IsPartial bool
}

// ReadStateSnapshot stores the latest known read state keyed by absolute file path.
type ReadStateSnapshot struct {
	// Files holds one read-state entry per file path.
	Files map[string]ReadState
}

// Lookup returns the recorded read state for one file path.
func (s ReadStateSnapshot) Lookup(path string) (ReadState, bool) {
	if path == "" || len(s.Files) == 0 {
		return ReadState{}, false
	}

	state, ok := s.Files[path]
	return state, ok
}

// UseContext carries the minimal host data shared with a single tool invocation.
type UseContext struct {
	// WorkingDir is the caller-visible working directory used to resolve relative paths.
	WorkingDir string
	// Invoker identifies the upstream component that initiated the tool call.
	Invoker string
	// ReadState stores the latest read snapshots that later write tools may consult.
	ReadState ReadStateSnapshot
}

// LookupReadState returns the recorded read state for one file path from the invocation context.
func (c UseContext) LookupReadState(path string) (ReadState, bool) {
	return c.ReadState.Lookup(path)
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
