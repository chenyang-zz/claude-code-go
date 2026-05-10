package analytics

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// FirstPartyEventLoggingEvent is a single event in the 1P event logging payload.
type FirstPartyEventLoggingEvent struct {
	EventType string `json:"event_type"`
	EventData any    `json:"event_data"`
}

// FirstPartyEventLoggingPayload is the request body sent to the batch endpoint.
type FirstPartyEventLoggingPayload struct {
	Events []FirstPartyEventLoggingEvent `json:"events"`
}

// FirstPartyEventLoggingExporterConfig controls the exporter's behaviour.
type FirstPartyEventLoggingExporterConfig struct {
	// BaseURL is the API base URL (e.g. "https://api.anthropic.com").
	BaseURL string
	// Path is the API path (default "/api/event_logging/batch").
	Path string
	// Timeout is the HTTP request timeout (default 10s).
	Timeout time.Duration
	// MaxBatchSize is the maximum number of events per batch (default 200).
	MaxBatchSize int
	// MaxAttempts is the maximum number of retry attempts (default 8).
	MaxAttempts int
	// BaseBackoff is the base backoff duration for quadratic backoff (default 500ms).
	BaseBackoff time.Duration
	// MaxBackoff is the maximum backoff duration (default 30s).
	MaxBackoff time.Duration
	// BatchDelay is the delay between batch chunks (default 100ms).
	BatchDelay time.Duration
	// SkipAuth skips authentication headers when true.
	SkipAuth bool
	// StorageDir is the directory for failed event persistence.
	// Defaults to ~/.claude/telemetry/.
	StorageDir string
	// SessionID identifies the current session, used in file naming.
	SessionID string
	// IsKilled is an optional probe checked before each POST. When it returns
	// true, the exporter skips sending and queues events to disk.
	IsKilled func() bool
	// Logger for debug output. When nil, no debug logging is performed.
	Logger *slog.Logger
}

// setDefaults fills in zero-valued fields with sensible defaults.
func (c *FirstPartyEventLoggingExporterConfig) setDefaults() {
	if c.BaseURL == "" {
		c.BaseURL = "https://api.anthropic.com"
	}
	if c.Path == "" {
		c.Path = "/api/event_logging/batch"
	}
	if c.Timeout <= 0 {
		c.Timeout = 10 * time.Second
	}
	if c.MaxBatchSize <= 0 {
		c.MaxBatchSize = 200
	}
	if c.MaxAttempts <= 0 {
		c.MaxAttempts = 8
	}
	if c.BaseBackoff <= 0 {
		c.BaseBackoff = 500 * time.Millisecond
	}
	if c.MaxBackoff <= 0 {
		c.MaxBackoff = 30 * time.Second
	}
	if c.BatchDelay <= 0 {
		c.BatchDelay = 100 * time.Millisecond
	}
	if c.StorageDir == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			c.StorageDir = filepath.Join(home, ".claude", "telemetry")
		}
	}
}

// FirstPartyEventLoggingExporter sends 1P events to the batch endpoint with
// disk-backed retry and quadratic backoff.
type FirstPartyEventLoggingExporter struct {
	cfg    FirstPartyEventLoggingExporterConfig
	client *http.Client

	mu         sync.Mutex
	attempts   int
	cancelFn   context.CancelFunc
	isRetrying atomic.Bool
	shutdown   atomic.Bool

	batchUUID string
}

// NewFirstPartyEventLoggingExporter creates a new exporter.
func NewFirstPartyEventLoggingExporter(cfg FirstPartyEventLoggingExporterConfig) *FirstPartyEventLoggingExporter {
	cfg.setDefaults()
	return &FirstPartyEventLoggingExporter{
		cfg: cfg,
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
		batchUUID: fmt.Sprintf("%d", time.Now().UnixNano()),
	}
}

// endpoint returns the full URL for the batch endpoint.
func (e *FirstPartyEventLoggingExporter) endpoint() string {
	return strings.TrimRight(e.cfg.BaseURL, "/") + "/" + strings.TrimLeft(e.cfg.Path, "/")
}

// Export sends events to the batch endpoint, queuing failures to disk and
// scheduling a backoff retry. Returns the number of events that failed.
func (e *FirstPartyEventLoggingExporter) Export(ctx context.Context, events []FirstPartyEventLoggingEvent) (int, error) {
	if e.shutdown.Load() {
		return len(events), fmt.Errorf("exporter is shut down")
	}
	if len(events) == 0 {
		return 0, nil
	}

	e.mu.Lock()
	attempts := e.attempts
	e.mu.Unlock()

	if attempts >= e.cfg.MaxAttempts {
		return len(events), fmt.Errorf("max attempts (%d) reached, dropping %d events", e.cfg.MaxAttempts, len(events))
	}

	// Check kill switch before sending
	if e.cfg.IsKilled != nil && e.cfg.IsKilled() {
		e.queueEvents(events)
		return len(events), fmt.Errorf("kill switch active")
	}

	failed, err := e.sendEventsInBatches(ctx, events)
	if len(failed) > 0 {
		e.queueEvents(failed)
		e.scheduleBackoffRetry(ctx)
	}

	// Success: reset backoff and immediately retry queued events
	if len(failed) == 0 {
		e.resetBackoff()
		if qc, _ := e.queuedEventCount(); qc > 0 {
			go e.retryFailedEvents(context.Background())
		}
	}

	return len(failed), err
}

// sendEventsInBatches chunks events and sends them sequentially. On first
// failure, remaining batches are queued without trying.
func (e *FirstPartyEventLoggingExporter) sendEventsInBatches(ctx context.Context, events []FirstPartyEventLoggingEvent) ([]FirstPartyEventLoggingEvent, error) {
	batches := chunkEvents(events, e.cfg.MaxBatchSize)

	var failedBatchEvents []FirstPartyEventLoggingEvent
	debug := e.cfg.Logger != nil

	for i, batch := range batches {
		if debug {
			e.cfg.Logger.Debug("1p exporter: sending batch",
				"batch", i+1, "total", len(batches), "size", len(batch))
		}

		if err := e.sendBatch(ctx, batch); err != nil {
			// Short-circuit: queue this batch and all remaining
			for j := i; j < len(batches); j++ {
				failedBatchEvents = append(failedBatchEvents, batches[j]...)
			}
			if debug {
				skipped := len(batches) - 1 - i
				e.cfg.Logger.Debug("1p exporter: batch failed, short-circuiting",
					"batch", i+1, "skipped", skipped, "error", err)
			}
			return failedBatchEvents, err
		}

		if i < len(batches)-1 && e.cfg.BatchDelay > 0 {
			select {
			case <-time.After(e.cfg.BatchDelay):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
	}

	return nil, nil
}

// sendBatch sends a single batch via HTTP POST.
func (e *FirstPartyEventLoggingExporter) sendBatch(ctx context.Context, batch []FirstPartyEventLoggingEvent) error {
	if e.cfg.IsKilled != nil && e.cfg.IsKilled() {
		return fmt.Errorf("firstParty sink killswitch active")
	}

	payload := FirstPartyEventLoggingPayload{Events: batch}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.endpoint(), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "claude-code")
	req.Header.Set("x-service-name", "claude-code")

	// Try with auth first when auth is not explicitly skipped.
	var authToken string
	if !e.cfg.SkipAuth {
		authToken = os.Getenv("ANTHROPIC_AUTH_TOKEN")
		if authToken != "" {
			req.Header.Set("Authorization", "Bearer "+authToken)
		}
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return fmt.Errorf("http do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		// Retry without auth (auth may have been rejected or not available)
		req2, err2 := http.NewRequestWithContext(ctx, http.MethodPost, e.endpoint(), bytes.NewReader(body))
		if err2 != nil {
			return fmt.Errorf("create retry request: %w", err2)
		}
		req2.Header.Set("Content-Type", "application/json")
		req2.Header.Set("User-Agent", "claude-code")
		req2.Header.Set("x-service-name", "claude-code")
		// Intentionally no Authorization header on the retry.

		resp2, err2 := e.client.Do(req2)
		if err2 != nil {
			return fmt.Errorf("http retry: %w", err2)
		}
		defer resp2.Body.Close()

		if resp2.StatusCode < 200 || resp2.StatusCode >= 300 {
			body2, _ := io.ReadAll(resp2.Body)
			return fmt.Errorf("retry status %d: %s", resp2.StatusCode, strings.TrimSpace(string(body2)))
		}
		return nil
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	return nil
}

// --- Disk persistence ---

// batchFilePath returns the path for the current batch's failed event file.
func (e *FirstPartyEventLoggingExporter) batchFilePath() string {
	filePrefix := "1p_failed_events."
	return filepath.Join(e.cfg.StorageDir, fmt.Sprintf("%s%s.%s.json", filePrefix, e.cfg.SessionID, e.batchUUID))
}

// queueEvents appends events to the disk-based failed event queue.
func (e *FirstPartyEventLoggingExporter) queueEvents(events []FirstPartyEventLoggingEvent) {
	if len(events) == 0 {
		return
	}

	// Ensure storage directory exists
	if err := os.MkdirAll(e.cfg.StorageDir, 0755); err != nil {
		return
	}

	fp := e.batchFilePath()
	f, err := os.OpenFile(fp, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	for _, evt := range events {
		data, err := json.Marshal(evt)
		if err != nil {
			continue
		}
		f.Write(data)
		f.Write([]byte("\n"))
	}
}

// loadEvents loads events from a JSONL file.
func (e *FirstPartyEventLoggingExporter) loadEvents(fp string) ([]FirstPartyEventLoggingEvent, error) {
	data, err := os.ReadFile(fp)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	events := make([]FirstPartyEventLoggingEvent, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			continue
		}
		var evt FirstPartyEventLoggingEvent
		if err := json.Unmarshal([]byte(line), &evt); err != nil {
			continue
		}
		events = append(events, evt)
	}
	return events, nil
}

// saveEvents overwrites a file with the given events as JSONL.
func (e *FirstPartyEventLoggingExporter) saveEvents(fp string, events []FirstPartyEventLoggingEvent) error {
	if len(events) == 0 {
		os.Remove(fp)
		return nil
	}

	if err := os.MkdirAll(e.cfg.StorageDir, 0755); err != nil {
		return err
	}

	var buf bytes.Buffer
	for _, evt := range events {
		data, err := json.Marshal(evt)
		if err != nil {
			continue
		}
		buf.Write(data)
		buf.Write([]byte("\n"))
	}
	return os.WriteFile(fp, buf.Bytes(), 0644)
}

// queuedEventCount returns the number of events persisted to disk.
func (e *FirstPartyEventLoggingExporter) queuedEventCount() (int, error) {
	events, err := e.loadEvents(e.batchFilePath())
	if err != nil {
		return 0, err
	}
	return len(events), nil
}

// --- Backoff retry ---

func (e *FirstPartyEventLoggingExporter) scheduleBackoffRetry(ctx context.Context) {
	if e.isRetrying.Load() || e.shutdown.Load() {
		return
	}

	e.mu.Lock()
	backoff := min(e.cfg.BaseBackoff*time.Duration(e.attempts*e.attempts), e.cfg.MaxBackoff)
	e.mu.Unlock()

	ctx, cancel := context.WithCancel(ctx)
	e.mu.Lock()
	e.cancelFn = cancel
	e.mu.Unlock()

	go func() {
		select {
		case <-time.After(backoff):
			if e.shutdown.Load() {
				return
			}
			e.retryFailedEvents(context.Background())
		case <-ctx.Done():
		}
	}()
}

func (e *FirstPartyEventLoggingExporter) retryFailedEvents(ctx context.Context) {
	if e.shutdown.Load() {
		return
	}

	fp := e.batchFilePath()

	for {
		if e.shutdown.Load() {
			return
		}

		events, err := e.loadEvents(fp)
		if err != nil || len(events) == 0 {
			return
		}

		e.mu.Lock()
		if e.attempts >= e.cfg.MaxAttempts {
			e.mu.Unlock()
			os.Remove(fp)
			e.resetBackoff()
			return
		}
		e.mu.Unlock()

		e.isRetrying.Store(true)

		// Clear file before retry
		os.Remove(fp)

		if e.cfg.Logger != nil {
			e.cfg.Logger.Debug("1p exporter: retrying failed events",
				"count", len(events), "attempt", e.loadAttempts()+1)
		}

		failed, _ := e.sendEventsInBatches(ctx, events)
		e.mu.Lock()
		e.attempts++
		e.mu.Unlock()

		e.isRetrying.Store(false)

		if len(failed) > 0 {
			e.saveEvents(fp, failed)
			e.scheduleBackoffRetry(ctx)
			return
		}

		e.resetBackoff()
	}
}

func (e *FirstPartyEventLoggingExporter) loadAttempts() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.attempts
}

func (e *FirstPartyEventLoggingExporter) resetBackoff() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.attempts = 0
	if e.cancelFn != nil {
		e.cancelFn()
		e.cancelFn = nil
	}
}

// Shutdown gracefully stops the exporter, flushing pending exports.
func (e *FirstPartyEventLoggingExporter) Shutdown(ctx context.Context) error {
	e.shutdown.Store(true)
	e.mu.Lock()
	if e.cancelFn != nil {
		e.cancelFn()
	}
	e.mu.Unlock()
	e.resetBackoff()
	return nil
}

// --- Helpers ---

func chunkEvents[T any](events []T, size int) [][]T {
	if size <= 0 {
		return [][]T{events}
	}
	var chunks [][]T
	for i := 0; i < len(events); i += size {
		end := min(i+size, len(events))
		chunks = append(chunks, events[i:end])
	}
	return chunks
}
