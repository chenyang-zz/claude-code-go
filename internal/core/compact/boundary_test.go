package compact

import (
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
)

func TestIsCompactBoundary_NilMessage(t *testing.T) {
	if IsCompactBoundary(nil) {
		t.Fatal("expected false for nil message")
	}
}

func TestIsCompactBoundary_NonBoundaryMessage(t *testing.T) {
	msg := message.Message{Role: message.RoleAssistant, Content: []message.ContentPart{
		message.TextPart("[compact_boundary]"),
	}}
	if IsCompactBoundary(&msg) {
		t.Fatal("expected false for unsupported role")
	}
}

func TestIsCompactBoundary_ValidBoundary(t *testing.T) {
	msg := message.Message{Role: message.RoleUser, Content: []message.ContentPart{
		message.TextPart("[compact_boundary]"),
		message.TextPart("auto"),
	}}
	if !IsCompactBoundary(&msg) {
		t.Fatal("expected true for valid boundary message")
	}
}

func TestIsCompactBoundary_LegacySystemBoundary(t *testing.T) {
	msg := message.Message{Role: message.RoleSystem, Content: []message.ContentPart{
		message.TextPart("[compact_boundary]"),
		message.TextPart("auto"),
	}}
	if !IsCompactBoundary(&msg) {
		t.Fatal("expected true for legacy system boundary message")
	}
}

func TestIsCompactBoundary_SystemWithoutBoundary(t *testing.T) {
	msg := message.Message{Role: message.RoleSystem, Content: []message.ContentPart{
		message.TextPart("some other system message"),
	}}
	if IsCompactBoundary(&msg) {
		t.Fatal("expected false for system message without boundary marker")
	}
}

func TestCreateBoundaryMessage(t *testing.T) {
	msg := CreateBoundaryMessage(TriggerAuto, 50000, 20)
	if msg.Role != message.RoleUser {
		t.Fatalf("expected user role, got %s", msg.Role)
	}
	if len(msg.Content) != 2 {
		t.Fatalf("expected 2 content parts, got %d", len(msg.Content))
	}
	if msg.Content[0].Text != "[compact_boundary]" {
		t.Fatalf("expected boundary marker, got %q", msg.Content[0].Text)
	}
	if msg.Content[1].Text != "auto" {
		t.Fatalf("expected auto trigger, got %q", msg.Content[1].Text)
	}
}

func TestCreateSummaryMessage(t *testing.T) {
	msg := CreateSummaryMessage("test summary", "/path/to/transcript")
	if msg.Role != message.RoleUser {
		t.Fatalf("expected user role, got %s", msg.Role)
	}
	text := msg.Content[0].Text
	if len(text) == 0 {
		t.Fatal("expected non-empty summary text")
	}
}

func TestCreateSummaryMessage_NoTranscript(t *testing.T) {
	msg := CreateSummaryMessage("test summary", "")
	text := msg.Content[0].Text
	if len(text) == 0 {
		t.Fatal("expected non-empty summary text")
	}
}

func TestFindLastCompactBoundary_NoBoundary(t *testing.T) {
	msgs := []message.Message{
		{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("hello")}},
		{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("hi")}},
	}
	idx := FindLastCompactBoundary(msgs)
	if idx != -1 {
		t.Fatalf("expected -1, got %d", idx)
	}
}

func TestFindLastCompactBoundary_WithBoundary(t *testing.T) {
	boundary := CreateBoundaryMessage(TriggerAuto, 50000, 10)
	msgs := []message.Message{
		{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("hello")}},
		boundary,
		{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("new message")}},
	}
	idx := FindLastCompactBoundary(msgs)
	if idx != 1 {
		t.Fatalf("expected boundary at index 1, got %d", idx)
	}
}

func TestFindLastCompactBoundary_MultipleBoundaries(t *testing.T) {
	boundary1 := CreateBoundaryMessage(TriggerAuto, 50000, 10)
	boundary2 := CreateBoundaryMessage(TriggerManual, 30000, 5)
	msgs := []message.Message{
		boundary1,
		{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("msg1")}},
		boundary2,
		{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("msg2")}},
	}
	idx := FindLastCompactBoundary(msgs)
	if idx != 2 {
		t.Fatalf("expected last boundary at index 2, got %d", idx)
	}
}

func TestGetMessagesAfterCompactBoundary_NoBoundary(t *testing.T) {
	msgs := []message.Message{
		{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("hello")}},
	}
	result := GetMessagesAfterCompactBoundary(msgs)
	if len(result) != 1 {
		t.Fatalf("expected 1 message, got %d", len(result))
	}
}

func TestGetMessagesAfterCompactBoundary_WithBoundary(t *testing.T) {
	boundary := CreateBoundaryMessage(TriggerAuto, 50000, 10)
	msgs := []message.Message{
		{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("old")}},
		boundary,
		{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("new")}},
	}
	result := GetMessagesAfterCompactBoundary(msgs)
	if len(result) != 2 {
		t.Fatalf("expected 2 messages (boundary + new), got %d", len(result))
	}
}

func TestBuildPostCompactMessages(t *testing.T) {
	boundary := CreateBoundaryMessage(TriggerAuto, 50000, 10)
	summary := CreateSummaryMessage("summary text", "")
	result := BuildPostCompactMessages(boundary, summary)
	if len(result) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(result))
	}
	if !IsCompactBoundary(&result[0]) {
		t.Fatal("expected first message to be boundary")
	}
	if result[1].Role != message.RoleUser {
		t.Fatal("expected second message to be user (summary)")
	}
}

func TestFormatCompactTime(t *testing.T) {
	ts := FormatCompactTime()
	if ts == "" {
		t.Fatal("expected non-empty timestamp")
	}
}
