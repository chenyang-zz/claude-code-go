package mailbox

import (
	"os"
	"testing"
)

// ── MarkMessageAsReadByIndex ──────────────────────────────────────────────

func TestMarkMessageAsReadByIndex(t *testing.T) {
	tmpDir := t.TempDir()

	// Write two messages
	WriteToMailbox("bob", Message{From: "alice", Text: "msg1", Timestamp: "2026-05-04T10:00:00Z"}, "default", tmpDir)
	WriteToMailbox("bob", Message{From: "charlie", Text: "msg2", Timestamp: "2026-05-04T10:01:00Z"}, "default", tmpDir)

	// Mark only the first message as read
	if err := MarkMessageAsReadByIndex("bob", "default", tmpDir, 0); err != nil {
		t.Fatalf("MarkMessageAsReadByIndex: %v", err)
	}

	msgs, err := ReadMailbox("bob", "default", tmpDir)
	if err != nil {
		t.Fatalf("ReadMailbox: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if !msgs[0].Read {
		t.Error("message 0 should be read")
	}
	if msgs[1].Read {
		t.Error("message 1 should still be unread")
	}
}

func TestMarkMessageAsReadByIndex_OutOfBounds(t *testing.T) {
	tmpDir := t.TempDir()

	WriteToMailbox("bob", Message{From: "alice", Text: "msg1", Timestamp: "2026-05-04T10:00:00Z"}, "default", tmpDir)

	// Negative index should not error
	if err := MarkMessageAsReadByIndex("bob", "default", tmpDir, -1); err != nil {
		t.Errorf("MarkMessageAsReadByIndex with negative index should not error: %v", err)
	}
	// Out-of-range index should not error
	if err := MarkMessageAsReadByIndex("bob", "default", tmpDir, 100); err != nil {
		t.Errorf("MarkMessageAsReadByIndex with out-of-range index should not error: %v", err)
	}
}

func TestMarkMessageAsReadByIndex_EmptyInbox(t *testing.T) {
	tmpDir := t.TempDir()

	if err := MarkMessageAsReadByIndex("nobody", "default", tmpDir, 0); err != nil {
		t.Errorf("MarkMessageAsReadByIndex on empty inbox: %v", err)
	}
}

func TestMarkMessageAsReadByIndex_AlreadyRead(t *testing.T) {
	tmpDir := t.TempDir()

	WriteToMailbox("bob", Message{From: "alice", Text: "msg1", Timestamp: "2026-05-04T10:00:00Z"}, "default", tmpDir)
	// Mark all as read first
	MarkMessagesAsRead("bob", "default", tmpDir)

	// Mark the (already read) message again
	if err := MarkMessageAsReadByIndex("bob", "default", tmpDir, 0); err != nil {
		t.Errorf("MarkMessageAsReadByIndex on already-read message: %v", err)
	}

	msgs, _ := ReadMailbox("bob", "default", tmpDir)
	if !msgs[0].Read {
		t.Error("message should still be read (idempotent)")
	}
}

// ── MarkMessagesAsReadByPredicate ─────────────────────────────────────────

func TestMarkMessagesAsReadByPredicate(t *testing.T) {
	tmpDir := t.TempDir()

	WriteToMailbox("bob", Message{From: "alice", Text: "from alice", Timestamp: "2026-05-04T10:00:00Z"}, "default", tmpDir)
	WriteToMailbox("bob", Message{From: "charlie", Text: "from charlie", Timestamp: "2026-05-04T10:01:00Z"}, "default", tmpDir)
	WriteToMailbox("bob", Message{From: "alice", Text: "second from alice", Timestamp: "2026-05-04T10:02:00Z"}, "default", tmpDir)

	// Mark only alice's messages as read
	if err := MarkMessagesAsReadByPredicate("bob", "default", tmpDir, func(m Message) bool {
		return m.From == "alice"
	}); err != nil {
		t.Fatalf("MarkMessagesAsReadByPredicate: %v", err)
	}

	msgs, _ := ReadMailbox("bob", "default", tmpDir)
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(msgs))
	}
	if !msgs[0].Read {
		t.Error("message 0 (alice) should be read")
	}
	if msgs[1].Read {
		t.Error("message 1 (charlie) should still be unread")
	}
	if !msgs[2].Read {
		t.Error("message 2 (alice) should be read")
	}
}

func TestMarkMessagesAsReadByPredicate_EmptyInbox(t *testing.T) {
	tmpDir := t.TempDir()

	if err := MarkMessagesAsReadByPredicate("nobody", "default", tmpDir, func(m Message) bool {
		return true
	}); err != nil {
		t.Errorf("MarkMessagesAsReadByPredicate on empty inbox: %v", err)
	}
}

func TestMarkMessagesAsReadByPredicate_NoMatch(t *testing.T) {
	tmpDir := t.TempDir()

	WriteToMailbox("bob", Message{From: "alice", Text: "msg1", Timestamp: "2026-05-04T10:00:00Z"}, "default", tmpDir)

	// Predicate matches nothing
	if err := MarkMessagesAsReadByPredicate("bob", "default", tmpDir, func(m Message) bool {
		return false
	}); err != nil {
		t.Fatalf("MarkMessagesAsReadByPredicate: %v", err)
	}

	msgs, _ := ReadMailbox("bob", "default", tmpDir)
	if msgs[0].Read {
		t.Error("message should still be unread (no predicate match)")
	}
}

// ── FormatTeammateMessages ────────────────────────────────────────────────

func TestFormatTeammateMessages_Single(t *testing.T) {
	msgs := []Message{
		{From: "alice", Text: "hello", Color: "red", Summary: "greeting"},
	}
	result := FormatTeammateMessages(msgs)
	expected := `<teammate-message teammate_id="alice" color="red" summary="greeting">` +
		"\nhello\n</teammate-message>"
	if result != expected {
		t.Errorf("FormatTeammateMessages = %q, want %q", result, expected)
	}
}

func TestFormatTeammateMessages_Multiple(t *testing.T) {
	msgs := []Message{
		{From: "alice", Text: "hi", Color: "red"},
		{From: "bob", Text: "hello", Color: "blue", Summary: "wave"},
	}
	result := FormatTeammateMessages(msgs)
	if result == "" {
		t.Fatal("expected non-empty result")
	}
	// Should be joined with double newlines
	if result[len(result)-1] != '>' {
		t.Error("result should end with closing tag")
	}
}

func TestFormatTeammateMessages_EmptySlice(t *testing.T) {
	result := FormatTeammateMessages([]Message{})
	if result != "" {
		t.Errorf("expected empty string for empty slice, got %q", result)
	}
}

func TestFormatTeammateMessages_NoOptionalFields(t *testing.T) {
	msgs := []Message{
		{From: "worker", Text: "task done"},
	}
	result := FormatTeammateMessages(msgs)
	expected := `<teammate-message teammate_id="worker">` + "\ntask done\n</teammate-message>"
	if result != expected {
		t.Errorf("FormatTeammateMessages = %q, want %q", result, expected)
	}
}

func TestFormatTeammateMessage_Helper(t *testing.T) {
	result := FormatTeammateMessage("bot", "completed", "green", "done")
	if result == "" {
		t.Fatal("expected non-empty result")
	}
	if result[:len("<teammate-message")] != "<teammate-message" {
		t.Errorf("result should start with <teammate-message, got %q", result[:30])
	}
}

// ── SendShutdownRequestToMailbox ──────────────────────────────────────────

func TestSendShutdownRequestToMailbox(t *testing.T) {
	tmpDir := t.TempDir()

	// Set up a name so sender resolution works
	os.Setenv("CLAUDE_CODE_AGENT_NAME", "leader")
	defer os.Unsetenv("CLAUDE_CODE_AGENT_NAME")

	requestID, err := SendShutdownRequestToMailbox("worker", "myteam", tmpDir, "task complete")
	if err != nil {
		t.Fatalf("SendShutdownRequestToMailbox: %v", err)
	}
	if requestID == "" {
		t.Error("expected non-empty request ID")
	}

	// Verify the message landed in the inbox
	msgs, err := ReadMailbox("worker", "myteam", tmpDir)
	if err != nil {
		t.Fatalf("ReadMailbox: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].From != "leader" {
		t.Errorf("From = %q, want leader", msgs[0].From)
	}

	// Verify it's a valid shutdown_request
	sr, ok := IsShutdownRequest(msgs[0].Text)
	if !ok {
		t.Fatal("message should be a shutdown_request")
	}
	if sr.From != "leader" {
		t.Errorf("shutdown from = %q, want leader", sr.From)
	}
	if sr.RequestID != requestID {
		t.Errorf("requestID = %q, want %q", sr.RequestID, requestID)
	}
}

func TestSendShutdownRequestToMailbox_DefaultName(t *testing.T) {
	tmpDir := t.TempDir()

	// No env var set — should default to "team-lead"
	os.Unsetenv("CLAUDE_CODE_AGENT_NAME")

	requestID, err := SendShutdownRequestToMailbox("worker", "myteam", tmpDir, "")
	if err != nil {
		t.Fatalf("SendShutdownRequestToMailbox: %v", err)
	}
	if requestID == "" {
		t.Error("expected non-empty request ID")
	}

	msgs, _ := ReadMailbox("worker", "myteam", tmpDir)
	if msgs[0].From != "team-lead" {
		t.Errorf("From = %q, want team-lead", msgs[0].From)
	}
}

// ── Integration: MarkByIndex then Predicate ───────────────────────────────

func TestMarkByIndexThenPredicate_Integration(t *testing.T) {
	tmpDir := t.TempDir()

	WriteToMailbox("bob", Message{From: "alice", Text: "m1", Timestamp: "2026-05-04T10:00:00Z"}, "default", tmpDir)
	WriteToMailbox("bob", Message{From: "bob", Text: "m2", Timestamp: "2026-05-04T10:01:00Z"}, "default", tmpDir)
	WriteToMailbox("bob", Message{From: "alice", Text: "m3", Timestamp: "2026-05-04T10:02:00Z"}, "default", tmpDir)

	// Mark by index first
	MarkMessageAsReadByIndex("bob", "default", tmpDir, 0)

	// Then mark remaining from bob by predicate
	MarkMessagesAsReadByPredicate("bob", "default", tmpDir, func(m Message) bool {
		return m.From == "bob"
	})

	msgs, _ := ReadMailbox("bob", "default", tmpDir)
	if !msgs[0].Read {
		t.Error("m0 (alice, by index) should be read")
	}
	if !msgs[1].Read {
		t.Error("m1 (bob, by predicate) should be read")
	}
	if msgs[2].Read {
		t.Error("m2 (alice, untouched) should be unread")
	}
}
