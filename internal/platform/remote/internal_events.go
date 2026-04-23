package remote

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// InternalEvent represents one event returned by the CCR v2 internal-events API.
// It mirrors the TypeScript InternalEvent shape used for session resume.
type InternalEvent struct {
	EventID       string         `json:"event_id"`
	EventType     string         `json:"event_type"`
	Payload       map[string]any `json:"payload"`
	EventMetadata map[string]any `json:"event_metadata,omitempty"`
	IsCompaction  bool           `json:"is_compaction"`
	CreatedAt     string         `json:"created_at"`
	AgentID       string         `json:"agent_id,omitempty"`
}

// IsSubagentEvent reports whether this event carries an agent_id, indicating it
// belongs to a subagent rather than the foreground session.
func (e InternalEvent) IsSubagentEvent() bool {
	return e.AgentID != ""
}

// listInternalEventsResponse mirrors the paginated API response shape.
type listInternalEventsResponse struct {
	Data       []InternalEvent `json:"data"`
	NextCursor string          `json:"next_cursor,omitempty"`
}

// InternalEventReader reads internal events from a remote CCR session.
type InternalEventReader interface {
	// ReadInternalEvents fetches foreground internal events.
	ReadInternalEvents(ctx context.Context) ([]InternalEvent, error)
	// ReadSubagentInternalEvents fetches subagent internal events.
	ReadSubagentInternalEvents(ctx context.Context) ([]InternalEvent, error)
}

// ReadInternalEvents fetches all foreground internal events from the CCR
// internal-events endpoint via paginated GET.
func (c *CCRClient) ReadInternalEvents(ctx context.Context) ([]InternalEvent, error) {
	return c.readPaginatedInternalEvents(ctx, "/worker/internal-events", nil)
}

// ReadSubagentInternalEvents fetches all subagent internal events from the CCR
// internal-events endpoint via paginated GET with ?subagents=true.
func (c *CCRClient) ReadSubagentInternalEvents(ctx context.Context) ([]InternalEvent, error) {
	return c.readPaginatedInternalEvents(ctx, "/worker/internal-events", map[string]string{"subagents": "true"})
}

// readPaginatedInternalEvents performs paginated GET against the given path.
func (c *CCRClient) readPaginatedInternalEvents(ctx context.Context, path string, params map[string]string) ([]InternalEvent, error) {
	if c == nil {
		return nil, fmt.Errorf("ccr client is nil")
	}

	baseURL, err := url.Parse(c.endpoint)
	if err != nil {
		return nil, fmt.Errorf("parse endpoint: %w", err)
	}

	var allEvents []InternalEvent
	cursor := ""
	pageCount := 0
	const maxPages = 100 // safety limit to prevent unbounded loops

	for {
		pageCount++
		if pageCount > maxPages {
			logger.WarnCF("remote_ccr", "internal events pagination exceeded safety limit", map[string]any{
				"pages": pageCount,
			})
			break
		}

		q := url.Values{}
		if cursor != "" {
			q.Set("cursor", cursor)
		}
		for k, v := range params {
			q.Set(k, v)
		}

		pageURL := baseURL.ResolveReference(&url.URL{Path: path, RawQuery: q.Encode()})

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL.String(), nil)
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}

		for k, v := range c.headers {
			req.Header.Set(k, v)
		}

		resp, err := c.client.Do(req)
		if err != nil {
			return nil, classifySendError(err)
		}

		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024))
		resp.Body.Close()

		if resp.StatusCode >= http.StatusBadRequest {
			return nil, classifyHTTPError(resp.StatusCode, string(body))
		}
		if readErr != nil {
			return nil, fmt.Errorf("read response body: %w", readErr)
		}

		var page listInternalEventsResponse
		if err := json.Unmarshal(body, &page); err != nil {
			return nil, fmt.Errorf("unmarshal response: %w", err)
		}

		allEvents = append(allEvents, page.Data...)

		logger.DebugCF("remote_ccr", "fetched internal events page", map[string]any{
			"path":         path,
			"page":         pageCount,
			"page_size":    len(page.Data),
			"total_so_far": len(allEvents),
		})

		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}

	logger.DebugCF("remote_ccr", "finished fetching internal events", map[string]any{
		"path":         path,
		"total_pages":  pageCount,
		"total_events": len(allEvents),
	})

	return allEvents, nil
}

// SubagentEvent wraps an InternalEvent with subagent-specific routing metadata.
type SubagentEvent struct {
	InternalEvent
}

// SubagentState tracks the known state of one subagent.
type SubagentState struct {
	// AgentID is the unique identifier for this subagent.
	AgentID string
	// AgentType is the type of agent (e.g. "general-purpose").
	AgentType string
	// Status is the current status: "active", "stopped", "error".
	Status string
	// EventCount tracks how many events have been observed for this subagent.
	EventCount int
	// LastEventAt is the timestamp of the most recent event.
	LastEventAt time.Time
}
