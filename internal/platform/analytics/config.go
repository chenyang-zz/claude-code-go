package analytics

// Config controls the behaviour of the analytics pipeline.
type Config struct {
	// Enabled gates all analytics event emission. When false, events are
	// silently dropped and no sink is started.
	Enabled bool
	// QueueSize sets the buffer capacity of the event channel. A larger
	// value reduces drops at the cost of memory. Zero selects the default (1024).
	QueueSize int
	// DatadogURL is the Datadog HTTP Logs intake URL. When non-empty, a
	// DatadogSink is created and wired to the emitter instead of ConsoleSink.
	DatadogURL string
	// DatadogAPIKey is the DD-API-KEY header value sent with each request.
	DatadogAPIKey string
	// UserType identifies the user tier (e.g. "ant", "external") and maps
	// to the "env" field in Datadog log entries.
	UserType string
}
