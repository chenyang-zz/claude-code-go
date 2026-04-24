package bash

import (
	"strings"
	"testing"
)

func TestBuildNotificationSummaryCompleted(t *testing.T) {
	summary := buildNotificationSummary("run tests", "completed", 0)
	if !strings.Contains(summary, "completed") {
		t.Fatalf("summary = %q, want 'completed'", summary)
	}
	if !strings.Contains(summary, "run tests") {
		t.Fatalf("summary = %q, want 'run tests'", summary)
	}
	if strings.Contains(summary, "exit code") {
		t.Fatal("summary should not contain exit code for success")
	}
}

func TestBuildNotificationSummaryCompletedWithExitCode(t *testing.T) {
	summary := buildNotificationSummary("run tests", "completed", 1)
	if !strings.Contains(summary, "completed") {
		t.Fatalf("summary = %q, want 'completed'", summary)
	}
	if !strings.Contains(summary, "exit code 1") {
		t.Fatalf("summary = %q, want 'exit code 1'", summary)
	}
}

func TestBuildNotificationSummaryFailed(t *testing.T) {
	summary := buildNotificationSummary("run tests", "failed", 7)
	if !strings.Contains(summary, "failed") {
		t.Fatalf("summary = %q, want 'failed'", summary)
	}
	if !strings.Contains(summary, "exit code 7") {
		t.Fatalf("summary = %q, want 'exit code 7'", summary)
	}
}

func TestBuildNotificationSummaryKilled(t *testing.T) {
	summary := buildNotificationSummary("run tests", "killed", -1)
	if !strings.Contains(summary, "stopped") {
		t.Fatalf("summary = %q, want 'stopped'", summary)
	}
}

func TestTaskNotificationPayloadString(t *testing.T) {
	p := taskNotificationPayload{
		TaskID:  "task_abc123",
		Status:  "completed",
		Summary: "Background command completed",
	}
	s := p.String()
	if !strings.Contains(s, "<task_notification>") {
		t.Fatal("missing task_notification tag")
	}
	if !strings.Contains(s, "<task_id>task_abc123</task_id>") {
		t.Fatal("missing task_id")
	}
	if !strings.Contains(s, "<status>completed</status>") {
		t.Fatal("missing status")
	}
	if !strings.Contains(s, "<summary>") {
		t.Fatal("missing summary")
	}
}

func TestTaskNotificationPayloadWithOutputPath(t *testing.T) {
	p := taskNotificationPayload{
		TaskID:     "task_abc123",
		Status:     "completed",
		Summary:    "Background command completed",
		OutputPath: "/tmp/output.txt",
	}
	s := p.String()
	if !strings.Contains(s, "<output_file>/tmp/output.txt</output_file>") {
		t.Fatal("missing output_file tag")
	}
}

func TestEscapeXMLString(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"hello", "hello"},
		{"a < b", "a &lt; b"},
		{"a > b", "a &gt; b"},
		{"a & b", "a &amp; b"},
		{`"hello"`, "&quot;hello&quot;"},
		{"'hello'", "&apos;hello&apos;"},
	}
	for _, c := range cases {
		got := escapeXMLString(c.in)
		if got != c.want {
			t.Fatalf("escapeXMLString(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

type testNotificationEmitter struct {
	calls []struct {
		TaskID     string
		Status     string
		Summary    string
		OutputPath string
	}
}

func (e *testNotificationEmitter) EmitTaskNotification(taskID string, status string, summary string, outputPath string) {
	e.calls = append(e.calls, struct {
		TaskID     string
		Status     string
		Summary    string
		OutputPath string
	}{taskID, status, summary, outputPath})
}

func TestEmitBackgroundCompletionNotification(t *testing.T) {
	emitter := &testNotificationEmitter{}
	emitBackgroundCompletionNotification(emitter, "task_1", "run tests", "completed", 0, "/tmp/out.txt")
	if len(emitter.calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(emitter.calls))
	}
	c := emitter.calls[0]
	if c.TaskID != "task_1" {
		t.Fatalf("TaskID = %q, want task_1", c.TaskID)
	}
	if c.Status != "completed" {
		t.Fatalf("Status = %q, want completed", c.Status)
	}
	if c.OutputPath != "/tmp/out.txt" {
		t.Fatalf("OutputPath = %q, want /tmp/out.txt", c.OutputPath)
	}
	if !strings.Contains(c.Summary, "run tests") {
		t.Fatalf("Summary = %q, want 'run tests'", c.Summary)
	}
}

func TestEmitBackgroundCompletionNotificationNilEmitter(t *testing.T) {
	// Should not panic.
	emitBackgroundCompletionNotification(nil, "task_1", "run tests", "completed", 0, "")
}

func TestBackgroundBashSummaryPrefix(t *testing.T) {
	summary := buildNotificationSummary("deploy", "completed", 0)
	if !strings.HasPrefix(summary, backgroundBashSummaryPrefix) {
		t.Fatalf("summary %q does not start with %q", summary, backgroundBashSummaryPrefix)
	}
}
