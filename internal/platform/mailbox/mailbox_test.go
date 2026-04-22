package mailbox

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestSanitizeName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello", "hello"},
		{"hello world", "hello-world"},
		{"hello/world", "hello-world"},
		{"hello@world.com", "hello-world-com"},
		{"hello.world", "hello-world"},
		{"hello_world", "hello_world"},
		{"hello-world", "hello-world"},
		{"Hello123", "Hello123"},
		{"", ""},
		{"///", "---"},
	}

	for _, tt := range tests {
		got := sanitizeName(tt.input)
		if got != tt.expected {
			t.Errorf("sanitizeName(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestGetInboxPath(t *testing.T) {
	home := "/home/user"
	path := getInboxPath("agent1", "team1", home)
	expected := filepath.Join(home, ".claude", "teams", "team1", "inboxes", "agent1.json")
	if path != expected {
		t.Errorf("getInboxPath() = %q, want %q", path, expected)
	}

	// Default team
	path = getInboxPath("agent1", "", home)
	expected = filepath.Join(home, ".claude", "teams", "default", "inboxes", "agent1.json")
	if path != expected {
		t.Errorf("getInboxPath(default team) = %q, want %q", path, expected)
	}

	// Sanitization
	path = getInboxPath("agent@1", "team/1", home)
	expected = filepath.Join(home, ".claude", "teams", "team-1", "inboxes", "agent-1.json")
	if path != expected {
		t.Errorf("getInboxPath(sanitized) = %q, want %q", path, expected)
	}
}

func TestReadMailbox_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	msgs, err := ReadMailbox("nobody", "default", tmpDir)
	if err != nil {
		t.Fatalf("ReadMailbox on non-existent inbox: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("expected empty slice, got %d messages", len(msgs))
	}
}

func TestWriteAndReadMailbox(t *testing.T) {
	tmpDir := t.TempDir()

	msg := Message{
		From:      "alice",
		Text:      "hello bob",
		Timestamp: "2026-04-23T10:00:00Z",
		Color:     "red",
	}

	if err := WriteToMailbox("bob", msg, "default", tmpDir); err != nil {
		t.Fatalf("WriteToMailbox: %v", err)
	}

	msgs, err := ReadMailbox("bob", "default", tmpDir)
	if err != nil {
		t.Fatalf("ReadMailbox: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].From != "alice" {
		t.Errorf("From = %q, want alice", msgs[0].From)
	}
	if msgs[0].Text != "hello bob" {
		t.Errorf("Text = %q, want 'hello bob'", msgs[0].Text)
	}
	if msgs[0].Read != false {
		t.Errorf("Read = %v, want false", msgs[0].Read)
	}
	if msgs[0].Color != "red" {
		t.Errorf("Color = %q, want red", msgs[0].Color)
	}
}

func TestWriteToMailbox_Append(t *testing.T) {
	tmpDir := t.TempDir()

	if err := WriteToMailbox("bob", Message{From: "alice", Text: "msg1", Timestamp: "2026-04-23T10:00:00Z"}, "default", tmpDir); err != nil {
		t.Fatalf("WriteToMailbox first: %v", err)
	}
	if err := WriteToMailbox("bob", Message{From: "charlie", Text: "msg2", Timestamp: "2026-04-23T10:01:00Z"}, "default", tmpDir); err != nil {
		t.Fatalf("WriteToMailbox second: %v", err)
	}

	msgs, err := ReadMailbox("bob", "default", tmpDir)
	if err != nil {
		t.Fatalf("ReadMailbox: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Text != "msg1" {
		t.Errorf("first message = %q, want msg1", msgs[0].Text)
	}
	if msgs[1].Text != "msg2" {
		t.Errorf("second message = %q, want msg2", msgs[1].Text)
	}
}

func TestReadUnreadMessages(t *testing.T) {
	tmpDir := t.TempDir()

	// Write two messages
	if err := WriteToMailbox("bob", Message{From: "alice", Text: "msg1", Timestamp: "2026-04-23T10:00:00Z"}, "default", tmpDir); err != nil {
		t.Fatalf("WriteToMailbox: %v", err)
	}
	if err := WriteToMailbox("bob", Message{From: "charlie", Text: "msg2", Timestamp: "2026-04-23T10:01:00Z"}, "default", tmpDir); err != nil {
		t.Fatalf("WriteToMailbox: %v", err)
	}

	// Both should be unread
	unread, err := ReadUnreadMessages("bob", "default", tmpDir)
	if err != nil {
		t.Fatalf("ReadUnreadMessages: %v", err)
	}
	if len(unread) != 2 {
		t.Fatalf("expected 2 unread, got %d", len(unread))
	}

	// Mark all as read
	if err := MarkMessagesAsRead("bob", "default", tmpDir); err != nil {
		t.Fatalf("MarkMessagesAsRead: %v", err)
	}

	unread, err = ReadUnreadMessages("bob", "default", tmpDir)
	if err != nil {
		t.Fatalf("ReadUnreadMessages after mark: %v", err)
	}
	if len(unread) != 0 {
		t.Errorf("expected 0 unread, got %d", len(unread))
	}
}

func TestClearMailbox(t *testing.T) {
	tmpDir := t.TempDir()

	// Write a message
	if err := WriteToMailbox("bob", Message{From: "alice", Text: "msg1", Timestamp: "2026-04-23T10:00:00Z"}, "default", tmpDir); err != nil {
		t.Fatalf("WriteToMailbox: %v", err)
	}

	// Clear
	if err := ClearMailbox("bob", "default", tmpDir); err != nil {
		t.Fatalf("ClearMailbox: %v", err)
	}

	msgs, err := ReadMailbox("bob", "default", tmpDir)
	if err != nil {
		t.Fatalf("ReadMailbox: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages after clear, got %d", len(msgs))
	}

	// Clear non-existent inbox should not error
	if err := ClearMailbox("nobody", "default", tmpDir); err != nil {
		t.Errorf("ClearMailbox on non-existent inbox: %v", err)
	}
}

func TestWriteToMailbox_Concurrent(t *testing.T) {
	tmpDir := t.TempDir()

	const numGoroutines = 10
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(i int) {
			defer wg.Done()
			msg := Message{
				From:      "sender",
				Text:      "message",
				Timestamp: "2026-04-23T10:00:00Z",
			}
			if err := WriteToMailbox("recipient", msg, "default", tmpDir); err != nil {
				t.Errorf("WriteToMailbox goroutine %d: %v", i, err)
			}
		}(i)
	}

	wg.Wait()

	msgs, err := ReadMailbox("recipient", "default", tmpDir)
	if err != nil {
		t.Fatalf("ReadMailbox: %v", err)
	}
	if len(msgs) != numGoroutines {
		t.Errorf("expected %d messages, got %d", numGoroutines, len(msgs))
	}
}

func TestMailbox_FileFormat(t *testing.T) {
	tmpDir := t.TempDir()

	msg := Message{
		From:      "alice",
		Text:      "hello",
		Timestamp: "2026-04-23T10:00:00Z",
		Read:      false,
		Color:     "blue",
		Summary:   "greeting",
	}
	if err := WriteToMailbox("bob", msg, "default", tmpDir); err != nil {
		t.Fatalf("WriteToMailbox: %v", err)
	}

	// Verify file is valid JSON array
	inboxPath := getInboxPath("bob", "default", tmpDir)
	data, err := os.ReadFile(inboxPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	var msgs []Message
	if err := json.Unmarshal(data, &msgs); err != nil {
		t.Fatalf("file is not valid JSON: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message in file, got %d", len(msgs))
	}
}

func TestMarkMessagesAsRead_EmptyInbox(t *testing.T) {
	tmpDir := t.TempDir()

	// Mark as read on non-existent inbox should not error
	if err := MarkMessagesAsRead("nobody", "default", tmpDir); err != nil {
		t.Errorf("MarkMessagesAsRead on empty inbox: %v", err)
	}
}

func TestWriteToMailbox_ReadAlwaysFalse(t *testing.T) {
	tmpDir := t.TempDir()

	// Write with Read=true should still be stored as Read=false
	msg := Message{
		From:      "alice",
		Text:      "hello",
		Timestamp: "2026-04-23T10:00:00Z",
		Read:      true, // intentionally set to true
	}
	if err := WriteToMailbox("bob", msg, "default", tmpDir); err != nil {
		t.Fatalf("WriteToMailbox: %v", err)
	}

	msgs, err := ReadMailbox("bob", "default", tmpDir)
	if err != nil {
		t.Fatalf("ReadMailbox: %v", err)
	}
	if len(msgs) != 1 || msgs[0].Read != false {
		t.Errorf("Read should be forced to false, got %v", msgs[0].Read)
	}
}
