package analytics

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

const (
	defaultLogsExportInterval = 10 * time.Second
	defaultMaxQueueSize       = 8192
	defaultMaxExportBatchSize = 200
)

// GrowthBookExperimentData holds GrowthBook experiment assignment data for logging.
type GrowthBookExperimentData struct {
	ExperimentID        string
	VariationID         int
	deviceID            string
	accountUUID         string
	organizationUUID    string
	SessionID           string
	UserAttributes      map[string]any
	ExperimentMetadata  map[string]any
}

// EventMetadata carries contextual data about an event being logged.
// This mirrors the TS core_metadata structure in a simplified form.
type EventMetadata struct {
	Model  string `json:"model,omitempty"`
	Betas  string `json:"betas,omitempty"`
	UserID string `json:"user_id,omitempty"`
}

// FirstPartyEventLoggerConfig controls the 1P event logger behaviour.
type FirstPartyEventLoggerConfig struct {
	// Enabled gates all 1P event logging.
	Enabled bool
	// ScheduledDelay is the interval between batch flushes (default 10s).
	ScheduledDelay time.Duration
	// MaxQueueSize is the maximum number of buffered events (default 8192).
	MaxQueueSize int
	// MaxBatchSize is the maximum events per export batch (default 200).
	MaxBatchSize int
	// SessionID identifies the current session.
	SessionID string
	// StorageDir is the directory for failed event persistence.
	// Defaults to ~/.claude/telemetry/.
	StorageDir string
	// BaseURL is the API base URL for the batch endpoint.
	BaseURL string
	// Path is the API path for the batch endpoint.
	Path string
	// SkipAuth skips authentication when true.
	SkipAuth bool
	// MaxAttempts is the maximum retry attempts for export (default 8).
	MaxAttempts int
	// IsKilled is an optional kill switch probe.
	IsKilled func() bool
	// Logger for debug output.
	Logger *slog.Logger
}

// FirstPartyEventLogger batches 1P events and periodically exports them
// via the FirstPartyEventLoggingExporter.
type FirstPartyEventLogger struct {
	cfg    FirstPartyEventLoggerConfig
	events chan FirstPartyEventLoggingEvent
	exporter *FirstPartyEventLoggingExporter
	done    chan struct{}
	wg      sync.WaitGroup
	closed  atomic.Bool
	mu      sync.Mutex

	shutdownOnce sync.Once
}

// NewFirstPartyEventLogger creates and starts a FirstPartyEventLogger.
// Returns nil if cfg.Enabled is false.
func NewFirstPartyEventLogger(cfg FirstPartyEventLoggerConfig) *FirstPartyEventLogger {
	if !cfg.Enabled {
		return nil
	}

	if cfg.ScheduledDelay <= 0 {
		cfg.ScheduledDelay = defaultLogsExportInterval
	}
	if cfg.MaxQueueSize <= 0 {
		cfg.MaxQueueSize = defaultMaxQueueSize
	}
	if cfg.MaxBatchSize <= 0 {
		cfg.MaxBatchSize = defaultMaxExportBatchSize
	}

	exporterCfg := FirstPartyEventLoggingExporterConfig{
		BaseURL:      cfg.BaseURL,
		Path:         cfg.Path,
		MaxBatchSize: cfg.MaxBatchSize,
		MaxAttempts:  cfg.MaxAttempts,
		SkipAuth:     cfg.SkipAuth,
		StorageDir:   cfg.StorageDir,
		SessionID:    cfg.SessionID,
		IsKilled:     cfg.IsKilled,
		Logger:       cfg.Logger,
	}

	logger := &FirstPartyEventLogger{
		cfg:      cfg,
		events:   make(chan FirstPartyEventLoggingEvent, cfg.MaxQueueSize),
		exporter: NewFirstPartyEventLoggingExporter(exporterCfg),
		done:     make(chan struct{}),
	}

	logger.wg.Add(1)
	go logger.drain()

	return logger
}

// drain reads events from the channel and periodically flushes them to the exporter.
func (l *FirstPartyEventLogger) drain() {
	defer l.wg.Done()

	ticker := time.NewTicker(l.cfg.ScheduledDelay)
	defer ticker.Stop()

	batch := make([]FirstPartyEventLoggingEvent, 0, l.cfg.MaxBatchSize)

	for {
		select {
		case evt, ok := <-l.events:
			if !ok {
				// Channel closed, flush remaining
				if len(batch) > 0 {
					l.flushBatch(batch)
				}
				return
			}
			batch = append(batch, evt)
			if len(batch) >= l.cfg.MaxBatchSize {
				l.flushBatch(batch)
				batch = batch[:0]
			}
		case <-ticker.C:
			if len(batch) > 0 {
				l.flushBatch(batch)
				batch = batch[:0]
			}
		case <-l.done:
			if len(batch) > 0 {
				l.flushBatch(batch)
			}
			return
		}
	}
}

// flushBatch sends a batch of events to the exporter.
func (l *FirstPartyEventLogger) flushBatch(batch []FirstPartyEventLoggingEvent) {
	if l.closed.Load() || len(batch) == 0 {
		return
	}

	// Copy the batch to avoid race with the drain loop
	events := make([]FirstPartyEventLoggingEvent, len(batch))
	copy(events, batch)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	failed, _ := l.exporter.Export(ctx, events)
	if failed > 0 && l.cfg.Logger != nil {
		l.cfg.Logger.Debug("1p logger: events failed to export",
			"failed", failed, "total", len(events))
	}
}

// enqueue tries to add an event to the buffer. Returns false if the buffer is full.
func (l *FirstPartyEventLogger) enqueue(evt FirstPartyEventLoggingEvent) bool {
	if l.closed.Load() {
		return false
	}
	select {
	case l.events <- evt:
		return true
	default:
		return false // queue full, drop
	}
}

// LogEvent logs a 1P event. Returns false if the event was dropped.
func (l *FirstPartyEventLogger) LogEvent(
	eventName string,
	eventID string,
	metadata EventMetadata,
	userMetadata map[string]any,
	eventMetadata map[string]any,
) bool {
	if l == nil || l.closed.Load() {
		return false
	}

	// Check kill switch
	if l.cfg.IsKilled != nil && l.cfg.IsKilled() {
		return false
	}

	// Build attributes for the event
	additional := make(map[string]any)
	for k, v := range metadata.toMap() {
		additional[k] = v
	}
	for k, v := range userMetadata {
		additional[k] = v
	}
	for k, v := range eventMetadata {
		additional[k] = v
	}

	evt := FirstPartyEventLoggingEvent{
		EventType: "ClaudeCodeInternalEvent",
		EventData: map[string]any{
			"event_name":   eventName,
			"event_id":     eventID,
			"session_id":   l.cfg.SessionID,
			"client_timestamp": time.Now().UTC().Format(time.RFC3339Nano),
			"event_metadata": additional,
		},
	}

	return l.enqueue(evt)
}

// LogGrowthBookExperiment logs a GrowthBook experiment assignment event.
func (l *FirstPartyEventLogger) LogGrowthBookExperiment(data GrowthBookExperimentData) bool {
	if l == nil || l.closed.Load() {
		return false
	}

	if l.cfg.IsKilled != nil && l.cfg.IsKilled() {
		return false
	}

	evtData := map[string]any{
		"event_id":       fmt.Sprintf("%d", time.Now().UnixNano()),
		"timestamp":      time.Now().UTC().Format(time.RFC3339Nano),
		"experiment_id":  data.ExperimentID,
		"variation_id":   data.VariationID,
		"environment":    "production",
	}

	if data.deviceID != "" {
		evtData["device_id"] = data.deviceID
	}
	if data.accountUUID != "" {
		evtData["account_uuid"] = data.accountUUID
	}
	if data.organizationUUID != "" {
		evtData["organization_uuid"] = data.organizationUUID
	}
	if data.SessionID != "" {
		evtData["session_id"] = data.SessionID
	}

	evt := FirstPartyEventLoggingEvent{
		EventType: "GrowthbookExperimentEvent",
		EventData: evtData,
	}

	return l.enqueue(evt)
}

// Shutdown gracefully stops the logger, flushing all pending events.
// Pass 0 to wait indefinitely.
func (l *FirstPartyEventLogger) Shutdown(timeout time.Duration) error {
	if l == nil {
		return nil
	}
	l.closed.Store(true)

	// Signal the drain loop (safe to call multiple times)
	l.shutdownOnce.Do(func() {
		close(l.done)
	})

	// Wait for drain with timeout
	waitCh := make(chan struct{})
	go func() {
		l.wg.Wait()
		close(waitCh)
	}()

	if timeout > 0 {
		select {
		case <-waitCh:
		case <-time.After(timeout):
		}
	} else {
		<-waitCh
	}

	// Shutdown exporter
	if l.exporter != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return l.exporter.Shutdown(ctx)
	}
	return nil
}

// metadataToMap converts EventMetadata to a flat map.
func (m EventMetadata) toMap() map[string]any {
	result := make(map[string]any)
	if m.Model != "" {
		result["model"] = m.Model
	}
	if m.Betas != "" {
		result["betas"] = m.Betas
	}
	if m.UserID != "" {
		result["user_id"] = m.UserID
	}
	return result
}
