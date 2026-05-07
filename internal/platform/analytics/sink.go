package analytics

import (
	"context"
	"log/slog"
)

// Sink receives analytics events for delivery to a backend.
// Implementations must be safe for concurrent use.
type Sink interface {
	// Emit delivers a single analytics event. Implementations should be
	// non-blocking where possible; any heavyweight work (network I/O, serialisation)
	// should be offloaded to a background goroutine.
	Emit(ctx context.Context, event Event) error
}

// ConsoleSink writes analytics events as structured log lines via slog.
// Intended for development / debugging and as the default sink when no
// external backend is configured.
type ConsoleSink struct {
	log *slog.Logger
}

// NewConsoleSink returns a ConsoleSink that writes to log.
func NewConsoleSink(log *slog.Logger) *ConsoleSink {
	return &ConsoleSink{log: log}
}

// Emit writes the event as a structured log line.
func (s *ConsoleSink) Emit(_ context.Context, event Event) error {
	attrs := []slog.Attr{
		slog.String("event", event.Name),
		slog.String("session", event.Metadata.SessionID),
		slog.Time("ts", event.Metadata.Timestamp),
	}
	for k, v := range event.Metadata.Labels {
		attrs = append(attrs, slog.Any(k, v))
	}
	// include a compact representation of the payload
	switch p := event.Payload.(type) {
	case ToolUsedEvent:
		attrs = append(attrs,
			slog.String("tool", p.ToolName),
			slog.Duration("dur", p.Duration),
			slog.Bool("ok", p.Success),
		)
	case SessionEvent:
		attrs = append(attrs,
			slog.String("action", p.Action),
			slog.Int("turns", p.TurnCount),
		)
	case CommandEvent:
		attrs = append(attrs,
			slog.String("cmd", p.CommandName),
			slog.Bool("ok", p.Success),
		)
	case ErrorEvent:
		attrs = append(attrs,
			slog.String("cat", p.Category),
			slog.String("err", p.ErrorType),
		)
	}
	s.log.LogAttrs(context.Background(), slog.LevelInfo, "analytics", attrs...)
	return nil
}
