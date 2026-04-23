package remote

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// SSEClient implements one receive-only SSE event stream with sequence-number
// tracking for deduplication and resumable reconnection.
type SSEClient struct {
	// response stores the active SSE HTTP response body.
	response *http.Response
	// cancel stops the internal read loop.
	cancel context.CancelFunc
	// streamCh carries decoded SSE events and terminal read errors.
	streamCh chan streamResult
	// closeOnce guarantees the stream is closed once.
	closeOnce sync.Once

	// seqMu protects sequence-number state.
	seqMu sync.RWMutex
	// lastSequenceNum is the high-water mark of sequence numbers seen on this stream.
	lastSequenceNum int64
	// seenSequenceNums tracks sequence numbers already observed for deduplication.
	seenSequenceNums map[int64]struct{}
}

type streamResult struct {
	event Event
	err   error
}

// DialSSE opens one SSE connection and starts the background frame reader.
// If initialSequenceNum is greater than zero, the request includes
// from_sequence_num query parameter and Last-Event-ID header so the server
// resumes from the last known position instead of replaying from the beginning.
func DialSSE(
	ctx context.Context,
	endpoint string,
	headers map[string]string,
	client *http.Client,
	initialSequenceNum int64,
) (*SSEClient, error) {
	trimmedEndpoint := strings.TrimSpace(endpoint)
	if trimmedEndpoint == "" {
		return nil, fmt.Errorf("missing sse endpoint")
	}
	if !strings.HasPrefix(trimmedEndpoint, "http://") && !strings.HasPrefix(trimmedEndpoint, "https://") {
		return nil, fmt.Errorf("invalid sse endpoint scheme: %s", trimmedEndpoint)
	}

	httpClient := client
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	// Build SSE URL with sequence number for resumption.
	sseURL := trimmedEndpoint
	if initialSequenceNum > 0 {
		u, err := url.Parse(trimmedEndpoint)
		if err == nil {
			q := u.Query()
			q.Set("from_sequence_num", strconv.FormatInt(initialSequenceNum, 10))
			u.RawQuery = q.Encode()
			sseURL = u.String()
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sseURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build sse request: %w", err)
	}
	req.Header.Set("accept", "text/event-stream")
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	if initialSequenceNum > 0 {
		req.Header.Set("Last-Event-ID", strconv.FormatInt(initialSequenceNum, 10))
	}

	logger.DebugCF("remote_sse", "opening sse stream", map[string]any{
		"endpoint":             trimmedEndpoint,
		"initial_sequence_num": initialSequenceNum,
	})

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("connect sse stream: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("sse stream rejected: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	streamCtx, cancel := context.WithCancel(context.Background())
	c := &SSEClient{
		response:         resp,
		cancel:           cancel,
		streamCh:         make(chan streamResult, 16),
		lastSequenceNum:  initialSequenceNum,
		seenSequenceNums: make(map[int64]struct{}),
	}
	go c.readLoop(streamCtx)

	logger.DebugCF("remote_sse", "sse stream connected", map[string]any{
		"endpoint":             trimmedEndpoint,
		"status":               resp.StatusCode,
		"initial_sequence_num": initialSequenceNum,
	})

	return c, nil
}

// Recv returns one decoded SSE event or one terminal stream error.
func (c *SSEClient) Recv(ctx context.Context) (Event, error) {
	if c == nil {
		return Event{}, ErrStreamClosed
	}

	select {
	case <-ctx.Done():
		return Event{}, ctx.Err()
	case result, ok := <-c.streamCh:
		if !ok {
			return Event{}, ErrStreamClosed
		}
		if result.err != nil {
			return Event{}, result.err
		}
		return result.event, nil
	}
}

// Close stops the read loop and releases the underlying HTTP response body.
func (c *SSEClient) Close() error {
	if c == nil {
		return nil
	}

	c.closeOnce.Do(func() {
		c.cancel()
		if c.response != nil && c.response.Body != nil {
			_ = c.response.Body.Close()
		}
	})
	return nil
}

// GetLastSequenceNum returns the high-water mark of sequence numbers seen on
// this stream. Callers that recreate the transport read this before close()
// and pass it as initialSequenceNum to the next instance so the server
// resumes from the right point instead of replaying everything.
func (c *SSEClient) GetLastSequenceNum() int64 {
	if c == nil {
		return 0
	}
	c.seqMu.RLock()
	defer c.seqMu.RUnlock()
	return c.lastSequenceNum
}

// isDuplicateSeqNum reports whether the given sequence number has already been
// observed on this stream. It is safe for concurrent use.
func (c *SSEClient) isDuplicateSeqNum(seqNum int64) bool {
	c.seqMu.RLock()
	defer c.seqMu.RUnlock()
	_, ok := c.seenSequenceNums[seqNum]
	return ok
}

// recordSequenceNum registers a newly observed sequence number, updates the
// high-water mark, and prunes old entries when the set grows too large.
func (c *SSEClient) recordSequenceNum(seqNum int64) {
	c.seqMu.Lock()
	defer c.seqMu.Unlock()

	c.seenSequenceNums[seqNum] = struct{}{}
	if seqNum > c.lastSequenceNum {
		c.lastSequenceNum = seqNum
	}

	// Prevent unbounded growth: prune sequence numbers well below the
	// high-water mark. Only numbers near lastSequenceNum matter for dedup.
	const maxSize = 1000
	const pruneThreshold = 200
	if len(c.seenSequenceNums) > maxSize {
		threshold := c.lastSequenceNum - int64(pruneThreshold)
		for s := range c.seenSequenceNums {
			if s < threshold {
				delete(c.seenSequenceNums, s)
			}
		}
	}
}

func (c *SSEClient) readLoop(ctx context.Context) {
	defer close(c.streamCh)
	defer func() {
		if c.response != nil && c.response.Body != nil {
			_ = c.response.Body.Close()
		}
	}()

	scanner := bufio.NewScanner(c.response.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var eventType string
	var eventID string
	dataLines := make([]string, 0, 4)

	flush := func() {
		if len(dataLines) == 0 {
			eventType = ""
			eventID = ""
			return
		}
		normalizedType := strings.TrimSpace(eventType)
		if normalizedType == "" {
			normalizedType = "message"
		}
		payload := strings.Join(dataLines, "\n")

		// Parse sequence number from the event ID, perform deduplication,
		// and update the high-water mark with pruning.
		if id := strings.TrimSpace(eventID); id != "" {
			if seqNum, err := strconv.ParseInt(id, 10, 64); err == nil {
				if c.isDuplicateSeqNum(seqNum) {
					logger.WarnCF("remote_sse", "duplicate sequence number detected", map[string]any{
						"seq_num": seqNum,
					})
				}
				c.recordSequenceNum(seqNum)
			}
		}

		select {
		case <-ctx.Done():
			return
		case c.streamCh <- streamResult{
			event: Event{
				Transport: TransportSSE,
				ID:        strings.TrimSpace(eventID),
				Type:      normalizedType,
				Data:      []byte(payload),
			},
		}:
		}
		eventType = ""
		eventID = ""
		dataLines = dataLines[:0]
	}

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		line := scanner.Text()
		if line == "" {
			flush()
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}

		field, value, hasField := strings.Cut(line, ":")
		if !hasField {
			continue
		}
		value = strings.TrimPrefix(value, " ")

		switch field {
		case "event":
			eventType = value
		case "id":
			eventID = value
		case "data":
			dataLines = append(dataLines, value)
		}
	}

	flush()

	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
		select {
		case <-ctx.Done():
			return
		case c.streamCh <- streamResult{
			err: fmt.Errorf("read sse stream: %w", err),
		}:
		}
	}
}
