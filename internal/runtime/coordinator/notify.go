package coordinator

import (
	"encoding/xml"
	"fmt"
	"strings"
)

// TaskNotification represents the XML structure of a task notification.
// This matches the TS coordinator mode's <task-notification> format.
type TaskNotification struct {
	XMLName xml.Name `xml:"task-notification"`
	TaskID  string   `xml:"task-id"`
	Status  string   `xml:"status"`
	Summary string   `xml:"summary"`
	Result  string   `xml:"result,omitempty"`
	Usage   *Usage   `xml:"usage,omitempty"`
}

// Usage holds token and timing statistics for a completed task.
type Usage struct {
	TotalTokens int `xml:"total_tokens"`
	ToolUses    int `xml:"tool_uses"`
	DurationMs  int `xml:"duration_ms"`
}

// FormatTaskNotification builds a <task-notification> XML string from a WorkerResult.
// The format matches the TS coordinator mode's documented notification format.
func FormatTaskNotification(result WorkerResult) string {
	if result.Worker == nil {
		return formatErrorNotification("", result.Error)
	}

	w := result.Worker
	status := workerStateToStatus(w.State)
	statusText := getStatusText(w.State, w.Input.Description)
	summary := fmt.Sprintf("Agent %q %s", w.Input.Description, statusText)

	notif := TaskNotification{
		TaskID:  w.ID,
		Status:  status,
		Summary: summary,
	}

	if result.Error == nil && w.State == WorkerStateCompleted {
		notif.Result = result.Output.Content
		notif.Usage = &Usage{
			TotalTokens: result.Output.TotalTokens,
			ToolUses:    result.Output.TotalToolUseCount,
			DurationMs:  result.Output.TotalDurationMs,
		}
	} else if result.Error != nil {
		notif.Result = result.Error.Error()
	}

	return marshalNotification(notif)
}

// FormatTaskNotificationFromWorker builds a notification from a Worker directly.
// Useful when the caller has a Worker but not a WorkerResult.
func FormatTaskNotificationFromWorker(w *Worker) string {
	if w == nil {
		return formatErrorNotification("", fmt.Errorf("nil worker"))
	}

	status := workerStateToStatus(w.State)
	statusText := getStatusText(w.State, w.Input.Description)
	summary := fmt.Sprintf("Agent %q %s", w.Input.Description, statusText)

	notif := TaskNotification{
		TaskID:  w.ID,
		Status:  status,
		Summary: summary,
	}

	if w.State == WorkerStateCompleted {
		notif.Result = w.Output.Content
		notif.Usage = &Usage{
			TotalTokens: w.Output.TotalTokens,
			ToolUses:    w.Output.TotalToolUseCount,
			DurationMs:  w.Output.TotalDurationMs,
		}
	} else if w.Error != nil {
		notif.Result = w.Error.Error()
	}

	return marshalNotification(notif)
}

// workerStateToStatus converts a WorkerState to the TS-compatible status string.
func workerStateToStatus(state WorkerState) string {
	switch state {
	case WorkerStateCompleted:
		return "completed"
	case WorkerStateFailed:
		return "failed"
	case WorkerStateStopped:
		return "killed"
	case WorkerStateRunning:
		return "running"
	case WorkerStateCreated:
		return "pending"
	default:
		return "unknown"
	}
}

// getStatusText returns human-readable status text matching TS format.
func getStatusText(state WorkerState, description string) string {
	switch state {
	case WorkerStateCompleted:
		return "completed"
	case WorkerStateFailed:
		return "failed"
	case WorkerStateStopped:
		return "was stopped"
	case WorkerStateRunning:
		return "is running"
	case WorkerStateCreated:
		return "is pending"
	default:
		return "has unknown status"
	}
}

// marshalNotification serializes a TaskNotification to XML string.
func marshalNotification(notif TaskNotification) string {
	var buf strings.Builder
	encoder := xml.NewEncoder(&buf)
	encoder.Indent("", "  ")

	if err := encoder.Encode(notif); err != nil {
		// Fallback to manual formatting if XML encoding fails
		return formatNotificationManual(notif)
	}

	return buf.String()
}

// formatNotificationManual builds XML manually as a fallback.
func formatNotificationManual(notif TaskNotification) string {
	var b strings.Builder
	b.WriteString("<task-notification>\n")
	b.WriteString(fmt.Sprintf("  <task-id>%s</task-id>\n", xmlEscape(notif.TaskID)))
	b.WriteString(fmt.Sprintf("  <status>%s</status>\n", xmlEscape(notif.Status)))
	b.WriteString(fmt.Sprintf("  <summary>%s</summary>\n", xmlEscape(notif.Summary)))
	if notif.Result != "" {
		b.WriteString(fmt.Sprintf("  <result>%s</result>\n", xmlEscape(notif.Result)))
	}
	if notif.Usage != nil {
		b.WriteString("  <usage>\n")
		b.WriteString(fmt.Sprintf("    <total_tokens>%d</total_tokens>\n", notif.Usage.TotalTokens))
		b.WriteString(fmt.Sprintf("    <tool_uses>%d</tool_uses>\n", notif.Usage.ToolUses))
		b.WriteString(fmt.Sprintf("    <duration_ms>%d</duration_ms>\n", notif.Usage.DurationMs))
		b.WriteString("  </usage>\n")
	}
	b.WriteString("</task-notification>")
	return b.String()
}

// formatErrorNotification builds an error notification when no worker is available.
func formatErrorNotification(taskID string, err error) string {
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}
	notif := TaskNotification{
		TaskID:  taskID,
		Status:  "failed",
		Summary: fmt.Sprintf("Task failed: %s", errMsg),
		Result:  errMsg,
	}
	return marshalNotification(notif)
}

// xmlEscape escapes special XML characters.
func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}
