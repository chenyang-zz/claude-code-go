package tool

import (
	"maps"
	"time"
)

// ReadState captures the minimal read snapshot needed to guard later writes.
type ReadState struct {
	// ReadAt records when the file content was last observed by a read tool.
	ReadAt time.Time
	// ObservedModTime records the file modification time seen by the latest successful read.
	ObservedModTime time.Time
	// ReadOffset stores the 1-based starting line for the last successful Read tool invocation.
	ReadOffset int
	// ReadLimit stores the requested line cap for the last successful Read tool invocation.
	ReadLimit int
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

// Clone returns a detached copy of the snapshot so callers can merge or expose state safely.
func (s ReadStateSnapshot) Clone() ReadStateSnapshot {
	if len(s.Files) == 0 {
		return ReadStateSnapshot{}
	}

	cloned := make(map[string]ReadState, len(s.Files))
	maps.Copy(cloned, s.Files)

	return ReadStateSnapshot{Files: cloned}
}

// Merge overlays another snapshot into the receiver, replacing entries for matching paths.
func (s *ReadStateSnapshot) Merge(other ReadStateSnapshot) {
	if s == nil || len(other.Files) == 0 {
		return
	}

	if s.Files == nil {
		s.Files = make(map[string]ReadState, len(other.Files))
	}

	for path, state := range other.Files {
		if path == "" {
			continue
		}
		s.Files[path] = state
	}
}

// UseContext carries the minimal host data shared with a single tool invocation.
type UseContext struct {
	// WorkingDir is the caller-visible working directory used to resolve relative paths.
	WorkingDir string
	// Invoker identifies the upstream component that initiated the tool call.
	Invoker string
	// ReadState stores the latest read snapshots that later write tools may consult.
	ReadState ReadStateSnapshot
	// SessionConfig carries the session-level configuration snapshot for dynamic prompt rendering.
	SessionConfig SessionConfigSnapshot
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
