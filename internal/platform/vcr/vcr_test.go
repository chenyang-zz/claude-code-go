package vcr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
	"github.com/sheepzhao/claude-code-go/internal/core/model"
)

// ---------------------------------------------------------------------------
// vcr.go tests
// ---------------------------------------------------------------------------

func TestIsEnvTruthy(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"", false},
		{"0", false},
		{"false", false},
		{"1", true},
		{"true", true},
		{"yes", true},
	}
	for _, tc := range tests {
		got := isEnvTruthy(tc.input)
		if got != tc.want {
			t.Errorf("isEnvTruthy(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// dehydrate.go tests
// ---------------------------------------------------------------------------

func TestDehydrateValue(t *testing.T) {
	cases := []struct {
		name  string
		input string
		check func(t *testing.T, out, input string)
	}{
		{
			name:  "num_files replaced",
			input: `num_files="42"`,
			check: func(t *testing.T, out, _ string) {
				if !strings.Contains(out, `num_files="[NUM]"`) {
					t.Errorf("expected num_files placeholder, got %q", out)
				}
			},
		},
		{
			name:  "duration_ms replaced",
			input: `duration_ms="5000"`,
			check: func(t *testing.T, out, _ string) {
				if !strings.Contains(out, `duration_ms="[DURATION]"`) {
					t.Errorf("expected duration_ms placeholder, got %q", out)
				}
			},
		},
		{
			name:  "cost_usd replaced",
			input: `cost_usd="0.01"`,
			check: func(t *testing.T, out, _ string) {
				if !strings.Contains(out, `cost_usd="[COST]"`) {
					t.Errorf("expected cost_usd placeholder, got %q", out)
				}
			},
		},
		{
			name:  "config home replaced",
			input: claudeConfigHome(),
			check: func(t *testing.T, out, _ string) {
				if strings.Contains(out, claudeConfigHome()) {
					t.Errorf("config home should be replaced, got %q", out)
				}
				if !strings.Contains(out, "[CONFIG_HOME]") {
					t.Errorf("expected [CONFIG_HOME] placeholder, got %q", out)
				}
			},
		},
		{
			name:  "cwd replaced",
			input: cwd(),
			check: func(t *testing.T, out, _ string) {
				if strings.Contains(out, cwd()) {
					t.Errorf("cwd should be replaced, got %q", out)
				}
				if !strings.Contains(out, "[CWD]") {
					t.Errorf("expected [CWD] placeholder, got %q", out)
				}
			},
		},
		{
			name:  "files modified by user",
			input: "Files modified by user: foo.txt, bar.go",
			check: func(t *testing.T, out, _ string) {
				if !strings.Contains(out, "[FILES]") {
					t.Errorf("expected [FILES] placeholder, got %q", out)
				}
			},
		},
		{
			name:  "available commands replaced",
			input: "Available commands: /help, /clear, /init",
			check: func(t *testing.T, out, _ string) {
				if !strings.Contains(out, "[COMMANDS]") {
					t.Errorf("expected [COMMANDS] placeholder, got %q", out)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := DehydrateValue(tc.input)
			tc.check(t, got, tc.input)
		})
	}
}

func TestHydrateValue(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		contains string
	}{
		{"num placeholder", "[NUM]", "1"},
		{"duration placeholder", "[DURATION]", "100"},
		{"config home placeholder", "[CONFIG_HOME]", claudeConfigHome()},
		{"cwd placeholder", "[CWD]", cwd()},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := HydrateValue(tc.input)
			if !strings.Contains(got, tc.contains) {
				t.Errorf("HydrateValue(%q) = %q, want contains %q",
					tc.input, got, tc.contains)
			}
		})
	}
}

func TestDehydrateTokenCountInput(t *testing.T) {
	input := "some data with UUID 550e8400-e29b-41d4-a716-446655440000 " +
		"and timestamp 2026-05-11T10:00:00Z"
	got := DehydrateTokenCountInput(input)
	if strings.Contains(got, "550e8400-e29b-41d4-a716-446655440000") {
		t.Errorf("UUID should be replaced, got %q", got)
	}
	if strings.Contains(got, "2026-05-11T10:00:00Z") {
		t.Errorf("timestamp should be replaced, got %q", got)
	}
	if !strings.Contains(got, "[UUID]") {
		t.Errorf("expected [UUID] placeholder, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// fixture.go tests
// ---------------------------------------------------------------------------

func TestHashInput(t *testing.T) {
	h1, err := hashInput("hello")
	if err != nil {
		t.Fatal(err)
	}
	h2, err := hashInput("hello")
	if err != nil {
		t.Fatal(err)
	}
	if h1 != h2 {
		t.Errorf("hashInput('hello') should be deterministic: %q vs %q", h1, h2)
	}
	if len(h1) != 12 {
		t.Errorf("hashInput('hello') = %q (len %d), want 12 chars", h1, len(h1))
	}
}

func TestWithFixtureRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("CLAUDE_CODE_TEST_FIXTURES_ROOT", tmpDir)

	name := "test-fixture"
	input := "test-input"

	t.Setenv("VCR_RECORD", "true")
	result1, err := WithFixture(name, input, func() (int, error) {
		return 42, nil
	})
	if err != nil {
		t.Fatalf("WithFixture (record) error = %v", err)
	}
	if result1 != 42 {
		t.Errorf("WithFixture (record) = %d, want 42", result1)
	}

	files, err := filepath.Glob(filepath.Join(tmpDir, "fixtures", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("expected at least one fixture file")
	}

	t.Setenv("VCR_RECORD", "")
	t.Setenv("VCR_ENABLED", "true")
	result2, err := WithFixture(name, input, func() (int, error) {
		return -1, nil
	})
	if err != nil {
		t.Fatalf("WithFixture (replay) error = %v", err)
	}
	if result2 != 42 {
		t.Errorf("WithFixture (replay) = %d, want 42 (cached)", result2)
	}
}

func TestWithFixtureMissing(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("CLAUDE_CODE_TEST_FIXTURES_ROOT", tmpDir)

	t.Setenv("VCR_RECORD", "")
	t.Setenv("VCR_ENABLED", "true")

	_, err := WithFixture("nonexistent", "some-input", func() (string, error) {
		return "should-not-be-called", nil
	})
	if err == nil {
		t.Fatal("expected error for missing fixture in replay mode")
	}
	if !strings.Contains(err.Error(), "fixture missing") {
		t.Errorf("expected 'fixture missing' error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// recorder.go tests
// ---------------------------------------------------------------------------

func TestRecorderPassthrough(t *testing.T) {
	recorder := NewTestRecorder("passthrough-test", func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Body:       http.NoBody,
			Header:     make(http.Header),
		}, nil
	})

	resp, err := recorder.RoundTrip(&http.Request{
		Method: "GET",
		Header: make(http.Header),
	})
	if err != nil {
		t.Fatalf("RoundTrip error = %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("StatusCode = %d, want 200", resp.StatusCode)
	}
}

func TestRecorderRecordReplay(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("CLAUDE_CODE_TEST_FIXTURES_ROOT", tmpDir)

	t.Setenv("VCR_RECORD", "true")

	interaction := RecordedInteraction{
		Request: RecordedRequest{
			Method: "POST",
			URL:    "https://api.test/echo",
			Header: map[string]string{"content-type": "application/json"},
			Body:   `{"hello":"world"}`,
		},
		Response: RecordedResponse{
			StatusCode: 200,
			Header:     map[string]string{"content-type": "application/json"},
			Body:       `{"reply":"ok"}`,
		},
	}

	hashRaw := RequestHashInput("POST", "https://api.test/echo", []byte(`{"hello":"world"}`))
	hash, err := hashInput(hashRaw)
	if err != nil {
		t.Fatal(err)
	}

	if err := writeFixture("test-api", hash, interaction); err != nil {
		t.Fatal(err)
	}

	t.Setenv("VCR_RECORD", "")
	t.Setenv("VCR_ENABLED", "true")

	recorder := NewRecorder("test-api", http.DefaultTransport)
	resp, err := recorder.RoundTrip(&http.Request{
		Method: "POST",
		URL:    &url.URL{Scheme: "https", Host: "api.test", Path: "/echo"},
		Header: http.Header{"Content-Type": {"application/json"}},
		Body:   io.NopCloser(strings.NewReader(`{"hello":"world"}`)),
	})
	if err != nil {
		t.Fatalf("RoundTrip replay error = %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("StatusCode = %d, want 200", resp.StatusCode)
	}

	var body bytes.Buffer
	_, err = body.ReadFrom(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatal(err)
	}

	var result map[string]string
	if err := json.Unmarshal(body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result["reply"] != "ok" {
		t.Errorf("body reply = %q, want ok", result["reply"])
	}
}

func TestRecordedRequestSanitization(t *testing.T) {
	r := &http.Request{
		Method: "POST",
		URL:    &url.URL{Scheme: "https", Host: "api.test", Path: "/login"},
		Header: http.Header{
			"X-Api-Key":     {"sk-secret-123"},
			"Content-Type":  {"application/json"},
			"Authorization": {"Bearer tok_xyz"},
		},
	}
	req := recordedRequest(r, []byte(`{"model":"claude"}`))
	if _, ok := req.Header["X-Api-Key"]; ok {
		t.Error("X-Api-Key should be stripped from recorded request headers")
	}
	if _, ok := req.Header["Authorization"]; ok {
		t.Error("Authorization should be stripped from recorded request headers")
	}
	if req.Header["Content-Type"] != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", req.Header["Content-Type"])
	}
}

// ---------------------------------------------------------------------------
// client.go tests
// ---------------------------------------------------------------------------

func TestWrapHTTPClientPassthrough(t *testing.T) {
	t.Setenv("VCR_ENABLED", "")

	inner := &http.Client{Timeout: 5 * time.Second}
	got := WrapHTTPClient("test", inner)

	if got != inner {
		t.Error("WrapHTTPClient should return inner client when VCR is disabled")
	}
}

func TestWrapHTTPClientEnabled(t *testing.T) {
	t.Setenv("VCR_ENABLED", "true")
	// Ensure VCR_RECORD is unset so we're in replay mode
	t.Setenv("VCR_RECORD", "")

	inner := &http.Client{Timeout: 10 * time.Second}
	got := WrapHTTPClient("wrap-test", inner)

	if got == inner {
		t.Error("WrapHTTPClient should return a new client when VCR is enabled")
	}
	if got.Timeout != inner.Timeout {
		t.Errorf("Timeout = %v, want %v", got.Timeout, inner.Timeout)
	}
}

func TestNewRecordClient(t *testing.T) {
	client := NewRecordClient("test-client")
	if client == nil {
		t.Fatal("NewRecordClient returned nil")
	}
	if client.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v, want 30s", client.Timeout)
	}
}

// ---------------------------------------------------------------------------
// Streaming recorder tests
// ---------------------------------------------------------------------------

// sseEvents is a sample SSE-formatted streaming response that simulates
// what the Anthropic API returns for a streaming text generation.
const sseEvents = `event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":" world"}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"output_tokens":15}}

event: message_stop
data: {"type":"message_stop"}
`

func TestRecorderRecordsSSEStreamingResponse(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("CLAUDE_CODE_TEST_FIXTURES_ROOT", tmpDir)
	t.Setenv("VCR_RECORD", "true")

	// Create a mock server that returns SSE events
	sseBytes := []byte(sseEvents)
	callCount := 0

	inner := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		callCount++
		return &http.Response{
			StatusCode: 200,
			Header: http.Header{
				"Content-Type": {"text/event-stream"},
			},
			Body: io.NopCloser(bytes.NewReader(sseBytes)),
		}, nil
	})

	recorder := NewRecorder("sse-test", inner)
	req := &http.Request{
		Method: "POST",
		URL:    &url.URL{Scheme: "https", Host: "api.anthropic.com", Path: "/v1/messages"},
		Header: http.Header{"Content-Type": {"application/json"}, "X-Api-Key": {"sk-test"}},
		Body:   io.NopCloser(strings.NewReader(`{"model":"claude-sonnet-4-5","max_tokens":100}`)),
	}

	resp, err := recorder.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip (record) error = %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("ReadAll (record) error = %v", err)
	}
	if string(body) != sseEvents {
		t.Errorf("Recorded body mismatch.\ngot:\n%s\nwant:\n%s", string(body), sseEvents)
	}
	if resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Errorf("Content-Type = %q, want text/event-stream", resp.Header.Get("Content-Type"))
	}
	if callCount != 1 {
		t.Errorf("inner transport called %d times, want 1", callCount)
	}

	// Now replay
	t.Setenv("VCR_RECORD", "")
	t.Setenv("VCR_ENABLED", "true")

	inner2 := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		callCount++
		return nil, nil // should not be called
	})

	recorder2 := NewRecorder("sse-test", inner2)
	req2 := &http.Request{
		Method: "POST",
		URL:    &url.URL{Scheme: "https", Host: "api.anthropic.com", Path: "/v1/messages"},
		Header: http.Header{"Content-Type": {"application/json"}},
		Body:   io.NopCloser(strings.NewReader(`{"model":"claude-sonnet-4-5","max_tokens":100}`)),
	}

	resp2, err := recorder2.RoundTrip(req2)
	if err != nil {
		t.Fatalf("RoundTrip (replay) error = %v", err)
	}
	defer resp2.Body.Close()

	body2, err := io.ReadAll(resp2.Body)
	if err != nil {
		t.Fatalf("ReadAll (replay) error = %v", err)
	}
	if string(body2) != sseEvents {
		t.Errorf("Replayed body mismatch.\ngot:\n%s\nwant:\n%s", string(body2), sseEvents)
	}
	if resp2.Header.Get("Content-Type") != "text/event-stream" {
		t.Errorf("Replay Content-Type = %q, want text/event-stream", resp2.Header.Get("Content-Type"))
	}
	if callCount != 1 {
		t.Errorf("inner2 transport called %d times (expected 0), want 1 (only from record)", callCount)
	}
}

// TestRecorderStreamRoundTrip verifies that the recorder correctly handles
// a multi-read streaming response (reading byte by byte, as an SSE parser would).
func TestRecorderStreamRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("CLAUDE_CODE_TEST_FIXTURES_ROOT", tmpDir)
	t.Setenv("VCR_RECORD", "true")

	// Record a short SSE response
	inner := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Header:     http.Header{"Content-Type": {"text/event-stream"}},
			Body:       io.NopCloser(bytes.NewReader([]byte(sseEvents))),
		}, nil
	})

	buf := &bytes.Buffer{}
	recorder := NewRecorder("sse-stream", inner)
	req := &http.Request{
		Method: "POST",
		URL:    &url.URL{Scheme: "https", Host: "api.anthropic.com", Path: "/v1/messages"},
		Header: http.Header{"Content-Type": {"application/json"}},
		Body:   io.NopCloser(strings.NewReader(`{"model":"claude","max_tokens":100}`)),
	}

	resp, err := recorder.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip (record) error = %v", err)
	}
	defer resp.Body.Close()

	// Read one byte at a time to simulate SSE incremental parsing
	oneByte := make([]byte, 1)
	for {
		_, err := resp.Body.Read(oneByte)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Read byte error = %v", err)
		}
		buf.Write(oneByte)
	}
	if buf.String() != sseEvents {
		t.Errorf("Byte-by-byte read mismatch.\ngot:\n%s\nwant:\n%s", buf.String(), sseEvents)
	}

	// Replay
	t.Setenv("VCR_RECORD", "")
	t.Setenv("VCR_ENABLED", "true")

	buf2 := &bytes.Buffer{}
	recorder2 := NewRecorder("sse-stream", http.DefaultTransport)
	req2 := &http.Request{
		Method: "POST",
		URL:    &url.URL{Scheme: "https", Host: "api.anthropic.com", Path: "/v1/messages"},
		Header: http.Header{"Content-Type": {"application/json"}},
		Body:   io.NopCloser(strings.NewReader(`{"model":"claude","max_tokens":100}`)),
	}

	resp2, err := recorder2.RoundTrip(req2)
	if err != nil {
		t.Fatalf("RoundTrip (replay) error = %v", err)
	}
	defer resp2.Body.Close()

	_, err = io.Copy(buf2, resp2.Body)
	if err != nil {
		t.Fatalf("io.Copy (replay) error = %v", err)
	}
	if buf2.String() != sseEvents {
		t.Errorf("Replayed body mismatch.\ngot:\n%s\nwant:\n%s", buf2.String(), sseEvents)
	}
}

// ---------------------------------------------------------------------------
// model_client_wrapper.go tests
// ---------------------------------------------------------------------------

// mockModelClient implements model.Client for WrapModelClient tests.
type mockModelClient struct {
	events []model.Event
	err    error
}

func (m *mockModelClient) Stream(_ context.Context, _ model.Request) (model.Stream, error) {
	if m.err != nil {
		return nil, m.err
	}
	return sliceToStream(m.events), nil
}

func TestWrapModelClientPassthrough(t *testing.T) {
	t.Setenv("VCR_ENABLED", "")
	t.Setenv("VCR_RECORD", "")

	mock := &mockModelClient{
		events: []model.Event{
			{Type: model.EventTypeTextDelta, Text: "hello"},
			{Type: model.EventTypeDone, StopReason: model.StopReasonEndTurn},
		},
	}
	wrapped := WrapModelClient("passthrough", mock)

	stream, err := wrapped.Stream(context.Background(), model.Request{
		Model: "claude-sonnet-4-5",
		Messages: []message.Message{
			{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("hi")}},
		},
	})
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	var got string
	for evt := range stream {
		if evt.Type == model.EventTypeTextDelta {
			got += evt.Text
		}
	}
	if got != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}
}

func TestWrapModelClientRecordReplay(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("CLAUDE_CODE_TEST_FIXTURES_ROOT", tmpDir)

	expectedEvents := []model.Event{
		{Type: model.EventTypeTextDelta, Text: "Hello from VCR"},
		{Type: model.EventTypeDone, StopReason: model.StopReasonEndTurn,
			Usage: &model.Usage{InputTokens: 10, OutputTokens: 5}},
	}

	// --- Record ---
	t.Setenv("VCR_RECORD", "true")
	t.Setenv("VCR_ENABLED", "")

	mock := &mockModelClient{events: expectedEvents}
	recorder := WrapModelClient("model-test", mock)

	stream1, err := recorder.Stream(context.Background(), model.Request{
		Model: "claude-sonnet-4-5",
		Messages: []message.Message{
			{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("say hi")}},
		},
	})
	if err != nil {
		t.Fatalf("Stream (record) error = %v", err)
	}

	var recordedEvents []model.Event
	for evt := range stream1 {
		recordedEvents = append(recordedEvents, evt)
	}
	if len(recordedEvents) != len(expectedEvents) {
		t.Fatalf("recorded %d events, want %d", len(recordedEvents), len(expectedEvents))
	}

	// --- Replay ---
	t.Setenv("VCR_RECORD", "")
	t.Setenv("VCR_ENABLED", "true")

	mockErr := &mockModelClient{err: fmt.Errorf("should not be called")}
	player := WrapModelClient("model-test", mockErr)

	stream2, err := player.Stream(context.Background(), model.Request{
		Model: "claude-sonnet-4-5",
		Messages: []message.Message{
			{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("say hi")}},
		},
	})
	if err != nil {
		t.Fatalf("Stream (replay) error = %v", err)
	}

	var replayedEvents []model.Event
	for evt := range stream2 {
		replayedEvents = append(replayedEvents, evt)
	}
	if len(replayedEvents) != len(expectedEvents) {
		t.Fatalf("replayed %d events, want %d", len(replayedEvents), len(expectedEvents))
	}
	if replayedEvents[0].Text != "Hello from VCR" {
		t.Errorf("replayed text = %q, want %q", replayedEvents[0].Text, "Hello from VCR")
	}
	if replayedEvents[1].Usage.InputTokens != 10 {
		t.Errorf("replayed input_tokens = %d, want 10", replayedEvents[1].Usage.InputTokens)
	}
}

// ---------------------------------------------------------------------------
// Initialization guard
// ---------------------------------------------------------------------------

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
