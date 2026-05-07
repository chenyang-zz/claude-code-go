package analytics

import (
	"context"
	"sync"
	"time"
)

const defaultQueueSize = 1024

// Emitter provides the public API for emitting analytics events.
// Events are enqueued asynchronously and drained by a background goroutine
// that feeds them to the configured Sink. Emit is non-blocking: if the
// internal queue is full, the event is discarded to avoid stalling the caller.
type Emitter struct {
	sink    Sink
	ch      chan Event
	wg      sync.WaitGroup
	closeMu sync.Mutex
	closed  bool
}

// NewEmitter creates an Emitter that delivers events to sink.
// If bufferSize <= 0, the default (1024) is used.
func NewEmitter(sink Sink, bufferSize int) *Emitter {
	if bufferSize <= 0 {
		bufferSize = defaultQueueSize
	}
	e := &Emitter{
		sink: sink,
		ch:   make(chan Event, bufferSize),
	}
	e.wg.Add(1)
	go e.drain()
	return e
}

// drain reads events from the channel and emits them via the sink.
func (e *Emitter) drain() {
	defer e.wg.Done()
	ctx := context.Background()
	for evt := range e.ch {
		// best-effort: log and continue on error
		_ = e.sink.Emit(ctx, evt)
	}
}

// Close stops the background drain goroutine. After Close returns no more
// events will be delivered to the sink. Safe to call multiple times.
func (e *Emitter) Close() {
	e.closeMu.Lock()
	if !e.closed {
		e.closed = true
		close(e.ch)
	}
	e.closeMu.Unlock()
	e.wg.Wait()
}

// enqueue tries to send evt to the channel without blocking.
// Returns false if the queue is full. Safe under concurrent Close:
// the lock is held through the send, preventing Close from closing
// the channel between the closed check and the send.
func (e *Emitter) enqueue(evt Event) bool {
	e.closeMu.Lock()
	defer e.closeMu.Unlock()
	if e.closed {
		return false
	}
	select {
	case e.ch <- evt:
		return true
	default:
		return false // queue full, drop
	}
}

// EmitToolUsed enqueues a tool usage event.
func (e *Emitter) EmitToolUsed(meta Metadata, toolName string, duration time.Duration, success bool, errMsg string) {
	e.enqueue(Event{
		Name:     EventToolUsed,
		Metadata: meta,
		Payload: ToolUsedEvent{
			ToolName: toolName,
			Duration: duration,
			Success:  success,
			ErrorMsg: errMsg,
		},
	})
}

// EmitSessionEvent enqueues a session lifecycle event.
func (e *Emitter) EmitSessionEvent(meta Metadata, action string, turnCount, msgCount int) {
	e.enqueue(Event{
		Name:     EventSession,
		Metadata: meta,
		Payload: SessionEvent{
			Action:       action,
			TurnCount:    turnCount,
			MessageCount: msgCount,
		},
	})
}

// EmitCommandEvent enqueues a slash command execution event.
func (e *Emitter) EmitCommandEvent(meta Metadata, cmdName string, success bool, args []string) {
	e.enqueue(Event{
		Name:     EventCommand,
		Metadata: meta,
		Payload: CommandEvent{
			CommandName: cmdName,
			Success:     success,
			Args:        args,
		},
	})
}

// EmitErrorEvent enqueues an error event.
func (e *Emitter) EmitErrorEvent(meta Metadata, category, errType, toolName string, durationMs int64) {
	e.enqueue(Event{
		Name:     EventError,
		Metadata: meta,
		Payload: ErrorEvent{
			Category:   category,
			ErrorType:  errType,
			ToolName:   toolName,
			DurationMs: durationMs,
		},
	})
}

// EmitRaw enqueues a generic event with an arbitrary name and payload.
func (e *Emitter) EmitRaw(meta Metadata, name string, payload any) {
	e.enqueue(Event{
		Name:     name,
		Metadata: meta,
		Payload:  payload,
	})
}
