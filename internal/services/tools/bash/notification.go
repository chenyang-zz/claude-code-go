package bash

import (
	"fmt"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// NotificationEmitter describes the minimal interface used by the Bash tool to
// emit background-task completion notifications into the host event stream.
type NotificationEmitter interface {
	// EmitTaskNotification sends one task-notification event for a background Bash task.
	// status is one of "completed", "failed", or "killed".
	// summary is the human-readable summary line.
	// outputPath is the optional path to the task's captured output file.
	EmitTaskNotification(taskID string, status string, summary string, outputPath string)
}

// taskNotificationPayload builds the stable XML-like text payload used for
// background Bash task notifications. The format mirrors the TypeScript
// <task_notification> tag shape consumed by downstream collapse transforms.
type taskNotificationPayload struct {
	TaskID     string
	Status     string
	Summary    string
	OutputPath string
}

// String serializes the notification into the canonical tag format.
func (p taskNotificationPayload) String() string {
	const (
		taskNotificationTag = "task_notification"
		taskIDTag           = "task_id"
		statusTag           = "status"
		summaryTag          = "summary"
		outputFileTag       = "output_file"
	)

	var outputFileLine string
	if p.OutputPath != "" {
		outputFileLine = fmt.Sprintf("\n  <%s>%s</%s>", outputFileTag, p.OutputPath, outputFileTag)
	}

	return fmt.Sprintf("<%s>\n  <%s>%s</%s>\n  <%s>%s</%s>%s\n  <%s>%s</%s>\n</%s>",
		taskNotificationTag,
		taskIDTag, p.TaskID, taskIDTag,
		statusTag, p.Status, statusTag,
		outputFileLine,
		summaryTag, p.Summary, summaryTag,
		taskNotificationTag,
	)
}

// backgroundBashSummaryPrefix is the prefix used by downstream collapse logic
// to identify bash-kind background notifications.
const backgroundBashSummaryPrefix = "Background command "

// buildNotificationSummary returns the user-visible summary for one background
// Bash task notification, matching the TypeScript wording.
func buildNotificationSummary(description string, status string, exitCode int) string {
	switch status {
	case "completed":
		if exitCode != 0 {
			return fmt.Sprintf("%s%q completed (exit code %d)", backgroundBashSummaryPrefix, description, exitCode)
		}
		return fmt.Sprintf("%s%q completed", backgroundBashSummaryPrefix, description)
	case "failed":
		return fmt.Sprintf("%s%q failed with exit code %d", backgroundBashSummaryPrefix, description, exitCode)
	case "killed":
		return fmt.Sprintf("%s%q was stopped", backgroundBashSummaryPrefix, description)
	default:
		return fmt.Sprintf("%s%q finished (%s)", backgroundBashSummaryPrefix, description, status)
	}
}

// emitBackgroundCompletionNotification sends one notification when a background
// Bash task reaches a terminal state.
func emitBackgroundCompletionNotification(
	emitter NotificationEmitter,
	taskID string,
	description string,
	status string,
	exitCode int,
	outputPath string,
) {
	if emitter == nil {
		return
	}

	summary := buildNotificationSummary(description, status, exitCode)
	emitter.EmitTaskNotification(taskID, status, summary, outputPath)

	logger.DebugCF("bash_tool", "emitted background task notification", map[string]any{
		"task_id":   taskID,
		"status":    status,
		"exit_code": exitCode,
	})
}

// escapeXMLString replaces XML-special characters with entities so that
// arbitrary command descriptions do not break the notification tag format.
func escapeXMLString(s string) string {
	var out []rune
	for _, r := range s {
		switch r {
		case '<':
			out = append(out, []rune("&lt;")...)
		case '>':
			out = append(out, []rune("&gt;")...)
		case '&':
			out = append(out, []rune("&amp;")...)
		case '"':
			out = append(out, []rune("&quot;")...)
		case '\'':
			out = append(out, []rune("&apos;")...)
		default:
			out = append(out, r)
		}
	}
	return string(out)
}

// buildNotificationSummaryEscaped is like buildNotificationSummary but XML-escapes
// the description before embedding it in the summary string.
func buildNotificationSummaryEscaped(description string, status string, exitCode int) string {
	return buildNotificationSummary(escapeXMLString(description), status, exitCode)
}
