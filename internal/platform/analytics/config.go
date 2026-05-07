package analytics

// Config controls the behaviour of the analytics pipeline.
type Config struct {
	// Enabled gates all analytics event emission. When false, events are
	// silently dropped and no sink is started.
	Enabled bool
	// QueueSize sets the buffer capacity of the event channel. A larger
	// value reduces drops at the cost of memory. Zero selects the default (1024).
	QueueSize int
}
