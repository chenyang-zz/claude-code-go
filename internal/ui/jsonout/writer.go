package jsonout

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/event"
)

// Writer consumes runtime events and writes one JSON object per line to the configured output.
type Writer struct {
	// Output receives newline-delimited JSON event objects.
	Output io.Writer
}

// NewWriter builds a JSON writer that defaults to stdout.
func NewWriter(output io.Writer) *Writer {
	if output == nil {
		output = os.Stdout
	}
	return &Writer{Output: output}
}

// WriteEvent serializes one runtime event as a single JSON line.
func (w *Writer) WriteEvent(evt event.Event) error {
	data, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("jsonout: marshal event: %w", err)
	}
	if _, err := fmt.Fprintln(w.Output, string(data)); err != nil {
		return fmt.Errorf("jsonout: write event: %w", err)
	}
	return nil
}

// Consume drains one event stream and writes every event as JSON.
func (w *Writer) Consume(stream event.Stream) error {
	for evt := range stream {
		if err := w.WriteEvent(evt); err != nil {
			return err
		}
	}
	return nil
}

// RenderEvent implements the console.EventRenderer interface for JSON output mode.
func (w *Writer) RenderEvent(evt event.Event) error {
	return w.WriteEvent(evt)
}

// RenderLine implements the console.EventRenderer interface by wrapping plain text
// in a message-delta event so that NDJSON consumers receive structured output.
func (w *Writer) RenderLine(text string) error {
	return w.WriteEvent(event.Event{
		Type:      event.TypeMessageDelta,
		Timestamp: time.Now(),
		Payload:   event.MessageDeltaPayload{Text: text},
	})
}
