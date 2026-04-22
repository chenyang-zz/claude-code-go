package console

import "github.com/sheepzhao/claude-code-go/internal/core/event"

// EventRenderer abstracts event rendering for both console and JSON output modes.
type EventRenderer interface {
	// RenderEvent writes one runtime event to the output sink.
	RenderEvent(evt event.Event) error
	// RenderLine writes one standalone text line for REPL-owned placeholder output.
	RenderLine(text string) error
}
