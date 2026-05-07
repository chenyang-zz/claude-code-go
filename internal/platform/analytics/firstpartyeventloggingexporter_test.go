package analytics

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// testEvent creates a simple event for testing.
func testEvent(suffix string) FirstPartyEventLoggingEvent {
	return FirstPartyEventLoggingEvent{
		EventType: "ClaudeCodeInternalEvent",
		EventData: map[string]any{"event_name": "test." + suffix},
	}
}

func TestNewExporterDefaults(t *testing.T) {
	e := NewFirstPartyEventLoggingExporter(FirstPartyEventLoggingExporterConfig{})
	if e.cfg.BaseURL != "https://api.anthropic.com" {
		t.Errorf("expected default base URL, got %s", e.cfg.BaseURL)
	}
	if e.cfg.Path != "/api/event_logging/batch" {
		t.Errorf("expected default path, got %s", e.cfg.Path)
	}
	if e.cfg.Timeout != 10*time.Second {
		t.Errorf("expected default timeout 10s, got %v", e.cfg.Timeout)
	}
	if e.cfg.MaxAttempts != 8 {
		t.Errorf("expected default max attempts 8, got %d", e.cfg.MaxAttempts)
	}
	if e.cfg.MaxBatchSize != 200 {
		t.Errorf("expected default max batch size 200, got %d", e.cfg.MaxBatchSize)
	}
}

func TestExportEmptyEvents(t *testing.T) {
	e := NewFirstPartyEventLoggingExporter(FirstPartyEventLoggingExporterConfig{
		SkipAuth: true,
	})
	failed, err := e.Export(context.Background(), nil)
	if failed != 0 || err != nil {
		t.Errorf("expected 0 failed, nil err; got %d, %v", failed, err)
	}
	failed, err = e.Export(context.Background(), []FirstPartyEventLoggingEvent{})
	if failed != 0 || err != nil {
		t.Errorf("expected 0 failed, nil err; got %d, %v", failed, err)
	}
}

func TestExportShutdown(t *testing.T) {
	e := NewFirstPartyEventLoggingExporter(FirstPartyEventLoggingExporterConfig{})
	e.Shutdown(context.Background())

	failed, err := e.Export(context.Background(), []FirstPartyEventLoggingEvent{testEvent("x")})
	if failed != 1 || err == nil {
		t.Errorf("expected 1 failed after shutdown, got %d, %v", failed, err)
	}
}

func TestExportKillSwitch(t *testing.T) {
	var killed atomic.Bool
	killed.Store(true)

	storageDir := t.TempDir()
	e := NewFirstPartyEventLoggingExporter(FirstPartyEventLoggingExporterConfig{
		IsKilled:   func() bool { return killed.Load() },
		StorageDir: storageDir,
		SessionID:  "sess-kill",
	})

	events := []FirstPartyEventLoggingEvent{testEvent("a"), testEvent("b")}
	failed, err := e.Export(context.Background(), events)
	if failed != 2 || err == nil {
		t.Errorf("expected 2 failed with kill switch, got %d, %v", failed, err)
	}

	// Events should be queued to disk
	count, _ := e.queuedEventCount()
	if count != 2 {
		t.Errorf("expected 2 queued events, got %d", count)
	}
}

func TestExportMaxAttempts(t *testing.T) {
	e := NewFirstPartyEventLoggingExporter(FirstPartyEventLoggingExporterConfig{
		MaxAttempts: 1,
		SkipAuth:    true,
		StorageDir:  t.TempDir(),
		SessionID:   "sess-max",
	})

	e.mu.Lock()
	e.attempts = 1 // exceed max attempts
	e.mu.Unlock()

	events := []FirstPartyEventLoggingEvent{testEvent("a")}
	failed, err := e.Export(context.Background(), events)
	if failed != 1 || err == nil {
		t.Errorf("expected 1 failed (max attempts), got %d, %v", failed, err)
	}
}

func TestSendBatchSuccess(t *testing.T) {
	var gotBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected application/json, got %s", r.Header.Get("Content-Type"))
		}
		if r.Header.Get("x-service-name") != "claude-code" {
			t.Errorf("expected x-service-name=claude-code, got %s", r.Header.Get("x-service-name"))
		}
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	e := NewFirstPartyEventLoggingExporter(FirstPartyEventLoggingExporterConfig{
		BaseURL: server.URL,
		SkipAuth: true,
	})
	err := e.sendBatch(context.Background(), []FirstPartyEventLoggingEvent{testEvent("ok")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var payload FirstPartyEventLoggingPayload
	if err := json.Unmarshal(gotBody, &payload); err != nil {
		t.Fatalf("failed to unmarshal body: %v", err)
	}
	if len(payload.Events) != 1 {
		t.Errorf("expected 1 event, got %d", len(payload.Events))
	}
}

func TestSendBatchNon2xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("bad request"))
	}))
	defer server.Close()

	e := NewFirstPartyEventLoggingExporter(FirstPartyEventLoggingExporterConfig{
		BaseURL: server.URL,
	})
	err := e.sendBatch(context.Background(), []FirstPartyEventLoggingEvent{testEvent("fail")})
	if err == nil {
		t.Fatal("expected error for 400 response")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Errorf("expected error to mention 400, got %v", err)
	}
}

func TestSendBatch401RetrySuccess(t *testing.T) {
	var callCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		if callCount.Load() == 1 {
			// First call returns 401 to trigger retry without auth
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		// Second call succeeds
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	e := NewFirstPartyEventLoggingExporter(FirstPartyEventLoggingExporterConfig{
		BaseURL: server.URL,
	})
	err := e.sendBatch(context.Background(), []FirstPartyEventLoggingEvent{testEvent("auth")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount.Load() != 2 {
		t.Errorf("expected 2 requests (1 auth + 1 retry), got %d", callCount.Load())
	}
}

func TestSendBatch401RetryFails(t *testing.T) {
	var callCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	e := NewFirstPartyEventLoggingExporter(FirstPartyEventLoggingExporterConfig{
		BaseURL: server.URL,
	})
	err := e.sendBatch(context.Background(), []FirstPartyEventLoggingEvent{testEvent("fail")})
	if err == nil {
		t.Fatal("expected error when retry also 401")
	}
}

func TestSendBatchKillSwitch(t *testing.T) {
	e := NewFirstPartyEventLoggingExporter(FirstPartyEventLoggingExporterConfig{
		IsKilled: func() bool { return true },
	})
	err := e.sendBatch(context.Background(), []FirstPartyEventLoggingEvent{testEvent("killed")})
	if err == nil {
		t.Fatal("expected error when kill switch active")
	}
}

func TestSendEventsInBatchesChunking(t *testing.T) {
	var batchesSent atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		batchesSent.Add(1)
		// First batch fails
		if batchesSent.Load() == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	e := NewFirstPartyEventLoggingExporter(FirstPartyEventLoggingExporterConfig{
		BaseURL:      server.URL,
		MaxBatchSize: 2,
		BatchDelay:   1 * time.Millisecond,
	})

	events := make([]FirstPartyEventLoggingEvent, 5)
	for i := range events {
		events[i] = testEvent(fmt.Sprintf("e%d", i))
	}

	failed, err := e.sendEventsInBatches(context.Background(), events)
	if err == nil {
		t.Fatal("expected error from first batch failure")
	}
	// Should short-circuit: all events returned (the failed batch + remaining)
	if len(failed) != 5 {
		t.Errorf("expected 5 failed events (short-circuit includes failed batch), got %d", len(failed))
	}
}

func TestSendEventsInBatchesAllSuccess(t *testing.T) {
	var batchesSent atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		batchesSent.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	e := NewFirstPartyEventLoggingExporter(FirstPartyEventLoggingExporterConfig{
		BaseURL:      server.URL,
		MaxBatchSize: 2,
		BatchDelay:   1 * time.Millisecond,
		SkipAuth:     true,
	})

	events := make([]FirstPartyEventLoggingEvent, 4)
	for i := range events {
		events[i] = testEvent(fmt.Sprintf("e%d", i))
	}

	failed, err := e.sendEventsInBatches(context.Background(), events)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(failed) != 0 {
		t.Errorf("expected 0 failed events, got %d", len(failed))
	}
	if batchesSent.Load() != 2 {
		t.Errorf("expected 2 batches, got %d", batchesSent.Load())
	}
}

func TestSendEventsInBatchesContextCancel(t *testing.T) {
	e := NewFirstPartyEventLoggingExporter(FirstPartyEventLoggingExporterConfig{
		BaseURL:      "http://127.0.0.1:1",
		MaxBatchSize: 1,
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // immediately cancelled

	_, err := e.sendEventsInBatches(ctx, []FirstPartyEventLoggingEvent{testEvent("a")})
	if err == nil {
		t.Fatal("expected context cancelled error")
	}
}

func TestDiskPersistenceQueueAndLoad(t *testing.T) {
	storageDir := t.TempDir()
	e := NewFirstPartyEventLoggingExporter(FirstPartyEventLoggingExporterConfig{
		StorageDir: storageDir,
		SessionID:  "sess-disk",
	})

	events := []FirstPartyEventLoggingEvent{
		testEvent("a"),
		testEvent("b"),
		testEvent("c"),
	}

	e.queueEvents(events)

	loaded, err := e.loadEvents(e.batchFilePath())
	if err != nil {
		t.Fatalf("failed to load events: %v", err)
	}
	if len(loaded) != 3 {
		t.Errorf("expected 3 events from disk, got %d", len(loaded))
	}
}

func TestDiskPersistenceSaveEvents(t *testing.T) {
	storageDir := t.TempDir()
	e := NewFirstPartyEventLoggingExporter(FirstPartyEventLoggingExporterConfig{
		StorageDir: storageDir,
		SessionID:  "sess-save",
	})

	events := []FirstPartyEventLoggingEvent{
		testEvent("x"),
		testEvent("y"),
	}

	err := e.saveEvents(e.batchFilePath(), events)
	if err != nil {
		t.Fatalf("failed to save events: %v", err)
	}

	loaded, err := e.loadEvents(e.batchFilePath())
	if err != nil {
		t.Fatalf("failed to load saved events: %v", err)
	}
	if len(loaded) != 2 {
		t.Errorf("expected 2 events, got %d", len(loaded))
	}

	// Save empty list should remove file
	err = e.saveEvents(e.batchFilePath(), nil)
	if err != nil {
		t.Fatalf("failed to save empty events: %v", err)
	}
	if _, err := os.Stat(e.batchFilePath()); !os.IsNotExist(err) {
		t.Error("expected file to be removed after saving empty events")
	}
}

func TestDiskPersistenceQueueEmpty(t *testing.T) {
	storageDir := t.TempDir()
	e := NewFirstPartyEventLoggingExporter(FirstPartyEventLoggingExporterConfig{
		StorageDir: storageDir,
		SessionID:  "sess-empty",
	})

	// queueEvents with nil/empty should not create file
	e.queueEvents(nil)
	e.queueEvents([]FirstPartyEventLoggingEvent{})

	if _, err := os.Stat(e.batchFilePath()); !os.IsNotExist(err) {
		t.Error("expected no file created for empty events")
	}
}

func TestLoadEventsNonExistentFile(t *testing.T) {
	storageDir := t.TempDir()
	e := NewFirstPartyEventLoggingExporter(FirstPartyEventLoggingExporterConfig{
		StorageDir: storageDir,
		SessionID:  "sess-none",
	})

	events, err := e.loadEvents(filepath.Join(storageDir, "nonexistent.json"))
	if err != nil {
		t.Fatalf("expected nil error for nonexistent file, got %v", err)
	}
	if len(events) != 0 {
		t.Errorf("expected 0 events, got %d", len(events))
	}
}

func TestQueuedEventCount(t *testing.T) {
	storageDir := t.TempDir()
	e := NewFirstPartyEventLoggingExporter(FirstPartyEventLoggingExporterConfig{
		StorageDir: storageDir,
		SessionID:  "sess-count",
	})

	// No events yet
	count, err := e.queuedEventCount()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 queued events initially, got %d", count)
	}

	e.queueEvents([]FirstPartyEventLoggingEvent{testEvent("a"), testEvent("b")})
	count, _ = e.queuedEventCount()
	if count != 2 {
		t.Errorf("expected 2 queued events, got %d", count)
	}
}

func TestExportQueuedEventsRetriedAfterSuccess(t *testing.T) {
	var callCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	storageDir := t.TempDir()
	e := NewFirstPartyEventLoggingExporter(FirstPartyEventLoggingExporterConfig{
		BaseURL:    server.URL,
		StorageDir: storageDir,
		SessionID:  "sess-retry",
		SkipAuth:   true,
	})

	// Queue events on disk first
	e.queueEvents([]FirstPartyEventLoggingEvent{testEvent("queued1"), testEvent("queued2")})

	count, _ := e.queuedEventCount()
	if count != 2 {
		t.Fatalf("expected 2 queued events, got %d", count)
	}

	// Now send a fresh batch successfully - this should trigger retry of queued events
	failed, err := e.Export(context.Background(), []FirstPartyEventLoggingEvent{testEvent("fresh")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if failed != 0 {
		t.Errorf("expected 0 failed events, got %d", failed)
	}

	// Wait a bit for the retry goroutine to run
	time.Sleep(100 * time.Millisecond)

	// Queued events should have been retried
	count, _ = e.queuedEventCount()
	if count != 0 {
		t.Errorf("expected 0 queued events after retry, got %d", count)
	}
}

func TestExporterShutdown(t *testing.T) {
	e := NewFirstPartyEventLoggingExporter(FirstPartyEventLoggingExporterConfig{})

	if !e.shutdown.Load() {
		if err := e.Shutdown(context.Background()); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	if !e.shutdown.Load() {
		t.Error("expected shutdown flag to be set")
	}

	// Double shutdown should not panic
	if err := e.Shutdown(context.Background()); err != nil {
		t.Fatalf("double shutdown should not error: %v", err)
	}
}

func TestChunkEvents(t *testing.T) {
	events := make([]FirstPartyEventLoggingEvent, 10)
	for i := range events {
		events[i] = testEvent(fmt.Sprintf("e%d", i))
	}

	chunks := chunkEvents(events, 3)
	if len(chunks) != 4 {
		t.Errorf("expected 4 chunks of size 3, got %d", len(chunks))
	}
	if len(chunks[0]) != 3 || len(chunks[1]) != 3 || len(chunks[2]) != 3 || len(chunks[3]) != 1 {
		t.Errorf("unexpected chunk sizes: %v", len(chunks))
	}

	// Zero or negative size returns all events as one chunk
	chunks = chunkEvents(events, 0)
	if len(chunks) != 1 || len(chunks[0]) != 10 {
		t.Errorf("expected 1 chunk with 10 events for size=0, got %d chunks of sizes %v", len(chunks), len(chunks))
	}

	// Empty events returns no chunks
	chunks = chunkEvents([]FirstPartyEventLoggingEvent{}, 10)
	if len(chunks) != 0 {
		t.Errorf("expected 0 chunks for empty events, got %d", len(chunks))
	}
}

func TestRetryFailedEventsRespectsShutdown(t *testing.T) {
	storageDir := t.TempDir()
	e := NewFirstPartyEventLoggingExporter(FirstPartyEventLoggingExporterConfig{
		StorageDir: storageDir,
		SessionID:  "sess-shutdown",
	})
	e.Shutdown(context.Background())
	e.retryFailedEvents(context.Background())
}

func TestScheduleBackoffRetryRespectsIsRetrying(t *testing.T) {
	e := NewFirstPartyEventLoggingExporter(FirstPartyEventLoggingExporterConfig{})
	e.isRetrying.Store(true)
	e.scheduleBackoffRetry(context.Background())
	// Should not panic or schedule
}

func TestResetBackoff(t *testing.T) {
	e := NewFirstPartyEventLoggingExporter(FirstPartyEventLoggingExporterConfig{})
	e.mu.Lock()
	e.attempts = 5
	e.mu.Unlock()

	e.resetBackoff()

	attempts := e.loadAttempts()
	if attempts != 0 {
		t.Errorf("expected 0 attempts after reset, got %d", attempts)
	}
}

func TestExportFilePersistence(t *testing.T) {
	var callCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	storageDir := t.TempDir()
	e := NewFirstPartyEventLoggingExporter(FirstPartyEventLoggingExporterConfig{
		BaseURL:    server.URL,
		StorageDir: storageDir,
		SessionID:  "sess-file",
	})

	events := []FirstPartyEventLoggingEvent{testEvent("fail1"), testEvent("fail2")}
	failed, err := e.Export(context.Background(), events)
	if failed != 2 || err == nil {
		t.Errorf("expected 2 failed events, got %d, err=%v", failed, err)
	}

	// Events should be persisted to disk
	fp := filepath.Join(storageDir, fmt.Sprintf("1p_failed_events.sess-file.%s.json", e.batchUUID))
	if _, err := os.Stat(fp); os.IsNotExist(err) {
		t.Error("expected failed events file to exist on disk")
	}

	data, _ := os.ReadFile(fp)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 json lines, got %d", len(lines))
	}

	// Gracefully stop the exporter
	e.Shutdown(context.Background())
	// Clean up persisted file before temp dir removal
	os.Remove(fp)
}
