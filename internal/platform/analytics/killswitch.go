package analytics

import (
	"context"
	"sync/atomic"
)

// SinkName identifies an analytics backend sink for runtime control.
type SinkName string

const (
	// SinkDatadog identifies the Datadog HTTP sink.
	SinkDatadog SinkName = "datadog"
	// SinkFirstParty identifies the first-party telemetry sink.
	SinkFirstParty SinkName = "firstParty"
)

// KillSwitchSink wraps a Sink and checks a runtime-togglable killed state
// before forwarding events. When killed, events are silently dropped.
// Implements the Sink interface.
type KillSwitchSink struct {
	name   SinkName
	sink   Sink
	killed atomic.Bool
}

// NewKillSwitchSink creates a KillSwitchSink wrapping sink.
// The sink starts in the non-killed (active) state.
func NewKillSwitchSink(name SinkName, sink Sink) *KillSwitchSink {
	return &KillSwitchSink{name: name, sink: sink}
}

// SetKilled sets the killed state. When true, Emit silently drops events.
func (k *KillSwitchSink) SetKilled(killed bool) {
	k.killed.Store(killed)
}

// IsKilled returns the current killed state.
func (k *KillSwitchSink) IsKilled() bool {
	return k.killed.Load()
}

// Emit forwards the event to the wrapped sink unless the switch is killed.
func (k *KillSwitchSink) Emit(ctx context.Context, event Event) error {
	if k.killed.Load() {
		return nil
	}
	return k.sink.Emit(ctx, event)
}
