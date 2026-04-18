package console

import (
	"fmt"

	"github.com/sheepzhao/claude-code-go/internal/core/event"
)

// StreamRenderer turns runtime events into console output.
type StreamRenderer struct {
	// Printer owns the final console writes.
	Printer *Printer
}

// NewStreamRenderer builds a renderer with the provided printer.
func NewStreamRenderer(printer *Printer) *StreamRenderer {
	if printer == nil {
		printer = NewPrinter(nil)
	}

	return &StreamRenderer{Printer: printer}
}

// Render consumes one event stream until it closes.
func (r *StreamRenderer) Render(stream event.Stream) error {
	for evt := range stream {
		if err := r.RenderEvent(evt); err != nil {
			return err
		}
	}
	return nil
}

// RenderEvent writes one already-decoded runtime event to the console when it has visible output.
func (r *StreamRenderer) RenderEvent(evt event.Event) error {
	switch evt.Type {
	case event.TypeMessageDelta:
		payload, ok := evt.Payload.(event.MessageDeltaPayload)
		if !ok {
			return fmt.Errorf("message delta payload type mismatch")
		}
		if err := r.Printer.Print(payload.Text); err != nil {
			return err
		}
	case event.TypeError:
		payload, ok := evt.Payload.(event.ErrorPayload)
		if !ok {
			return fmt.Errorf("error payload type mismatch")
		}
		if err := r.Printer.PrintLine(payload.Message); err != nil {
			return err
		}
	case event.TypeToolCallStarted:
		payload, ok := evt.Payload.(event.ToolCallPayload)
		if !ok {
			return fmt.Errorf("tool call started payload type mismatch")
		}
		if err := r.Printer.PrintLine(fmt.Sprintf("  Tool started: %s", payload.Name)); err != nil {
			return err
		}
	case event.TypeToolCallFinished:
		payload, ok := evt.Payload.(event.ToolResultPayload)
		if !ok {
			return fmt.Errorf("tool call finished payload type mismatch")
		}
		suffix := ""
		if payload.IsError {
			suffix = " (error)"
		}
		if err := r.Printer.PrintLine(fmt.Sprintf("  Tool finished: %s%s", payload.Name, suffix)); err != nil {
			return err
		}
	case event.TypeApprovalRequired:
		payload, ok := evt.Payload.(event.ApprovalPayload)
		if !ok {
			return fmt.Errorf("approval required payload type mismatch")
		}
		label := payload.ToolName
		if payload.Action != "" && payload.Path != "" {
			label = fmt.Sprintf("%s wants to %s %s", payload.ToolName, payload.Action, payload.Path)
		}
		if err := r.Printer.PrintLine(fmt.Sprintf("  Approval required: %s", label)); err != nil {
			return err
		}
	}
	return nil
}

// RenderLine writes one standalone console line for REPL-owned placeholder output.
func (r *StreamRenderer) RenderLine(text string) error {
	return r.Printer.PrintLine(text)
}
