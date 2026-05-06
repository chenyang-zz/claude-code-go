package analytics

import (
	"log/slog"
)

// InitAnalytics creates and returns an Emitter wired to sink.
// If cfg.Enabled is false, it returns a no-op emitter that discards all events.
// Caller must call Close on the returned Emitter during shutdown.
func InitAnalytics(cfg Config, log *slog.Logger) *Emitter {
	if !cfg.Enabled {
		return newNoopEmitter()
	}

	sink := NewConsoleSink(log)
	return NewEmitter(sink, cfg.QueueSize)
}

// noopEmitter silently drops every event.
type noopEmitter struct{}

func newNoopEmitter() *Emitter {
	e := &Emitter{
		sink:   nil,
		ch:     nil,
		closed: true,
	}
	// mark the background goroutine as "done" so Close() returns immediately
	e.wg.Add(1)
	e.wg.Done()
	return e
}
