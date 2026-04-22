package remote

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// SSEClient implements one receive-only SSE event stream.
type SSEClient struct {
	// response stores the active SSE HTTP response body.
	response *http.Response
	// cancel stops the internal read loop.
	cancel context.CancelFunc
	// streamCh carries decoded SSE events and terminal read errors.
	streamCh chan streamResult
	// closeOnce guarantees the stream is closed once.
	closeOnce sync.Once
}

type streamResult struct {
	event Event
	err   error
}

// DialSSE opens one SSE connection and starts the background frame reader.
func DialSSE(
	ctx context.Context,
	endpoint string,
	headers map[string]string,
	client *http.Client,
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

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, trimmedEndpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build sse request: %w", err)
	}
	req.Header.Set("accept", "text/event-stream")
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	logger.DebugCF("remote_sse", "opening sse stream", map[string]any{
		"endpoint": trimmedEndpoint,
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
		response: resp,
		cancel:   cancel,
		streamCh: make(chan streamResult, 16),
	}
	go c.readLoop(streamCtx)

	logger.DebugCF("remote_sse", "sse stream connected", map[string]any{
		"endpoint": trimmedEndpoint,
		"status":   resp.StatusCode,
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
