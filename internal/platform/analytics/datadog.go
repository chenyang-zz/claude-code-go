package analytics

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

const (
	datadogDefaultTimeout = 5 * time.Second
	datadogService        = "claude-code"
	datadogHostname       = "claude-code"
	datadogSource         = "go"
)

// tagFields are metadata labels that should appear in the ddtags field
// of the Datadog log entry. All labels are included as top-level JSON
// keys regardless; tagFields controls which are additionally promoted
// to the ddtags comma-separated list for Datadog facet indexing.
var tagFields = map[string]bool{
	"arch":             true,
	"clientType":       true,
	"errorType":        true,
	"model":            true,
	"platform":         true,
	"provider":         true,
	"skillMode":        true,
	"subscriptionType": true,
	"toolName":         true,
	"userType":         true,
	"version":          true,
}

// DatadogSink sends analytics events to the Datadog HTTP Logs API.
// It implements the Sink interface, serialising each Event to the
// Datadog JSON format and POSTing it to the configured endpoint.
type DatadogSink struct {
	client   *http.Client
	endpoint string
	apiKey   string
	userType string
	log      *slog.Logger
}

// NewDatadogSink creates a DatadogSink that POSTs JSON-encoded events
// to the given endpoint. apiKey is sent as the DD-API-KEY header.
// userType becomes the "env" field in the Datadog log entry.
func NewDatadogSink(endpoint, apiKey, userType string, log *slog.Logger) *DatadogSink {
	return &DatadogSink{
		client: &http.Client{
			Timeout: datadogDefaultTimeout,
		},
		endpoint: endpoint,
		apiKey:   apiKey,
		userType: userType,
		log:      log,
	}
}

// Emit serialises the event and POSTs it to the Datadog Logs API.
// Returns nil on success; logs warnings on HTTP or network errors.
func (s *DatadogSink) Emit(ctx context.Context, event Event) error {
	body := s.buildPayload(event)

	jsonBytes, err := json.Marshal(body)
	if err != nil {
		s.log.Warn("datadog: failed to marshal event", "event", event.Name, "error", err)
		return fmt.Errorf("datadog: marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewReader(jsonBytes))
	if err != nil {
		s.log.Warn("datadog: failed to create request", "event", event.Name, "error", err)
		return fmt.Errorf("datadog: request: %w", err)
	}
	req.Header.Set("DD-API-KEY", s.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		s.log.Warn("datadog: request failed", "event", event.Name, "error", err)
		return fmt.Errorf("datadog: do: %w", err)
	}
	resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		s.log.Warn("datadog: unexpected status", "event", event.Name, "status", resp.StatusCode)
		return fmt.Errorf("datadog: status %d", resp.StatusCode)
	}

	return nil
}

// buildPayload constructs the Datadog JSON payload from an Event.
func (s *DatadogSink) buildPayload(event Event) map[string]any {
	var tags []string
	tags = append(tags, "event:"+event.Name)
	for k, v := range event.Metadata.Labels {
		if tagFields[k] {
			tags = append(tags, fmt.Sprintf("%s:%v", k, v))
		}
	}

	payload := map[string]any{
		"ddsource": datadogSource,
		"ddtags":   strings.Join(tags, ","),
		"message":  event.Name,
		"service":  datadogService,
		"hostname": datadogHostname,
		"env":      s.userType,
	}

	// Merge all metadata labels as top-level keys
	for k, v := range event.Metadata.Labels {
		payload[k] = v
	}

	// Enrich with event-type-specific fields from the typed payload
	switch p := event.Payload.(type) {
	case ToolUsedEvent:
		payload["toolName"] = p.ToolName
		payload["duration"] = p.Duration.String()
		payload["success"] = p.Success
		if p.ErrorMsg != "" {
			payload["error"] = p.ErrorMsg
		}
	case ErrorEvent:
		payload["errorCategory"] = p.Category
		payload["errorType"] = p.ErrorType
		payload["toolName"] = p.ToolName
	case CommandEvent:
		payload["commandName"] = p.CommandName
		payload["success"] = p.Success
	case SessionEvent:
		payload["sessionAction"] = p.Action
	case map[string]any:
		for k, v := range p {
			payload[k] = v
		}
	}

	return payload
}
