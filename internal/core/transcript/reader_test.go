package transcript

import (
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
)

func TestNewReader_FileNotFound(t *testing.T) {
	_, err := NewReader("/nonexistent/path/transcript.jsonl")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestReader_ReadNext_AllEntryTypes(t *testing.T) {
	lines := []string{
		`{"type":"user","timestamp":"2026-04-24T10:00:00Z","message":{"role":"user","content":[{"type":"text","text":"hello"}]}}`,
		`{"type":"assistant","timestamp":"2026-04-24T10:00:00Z","message":{"role":"assistant","content":[{"type":"text","text":"hi there"}]}}`,
		`{"type":"tool_use","timestamp":"2026-04-24T10:00:00Z","tool_use_id":"tu1","name":"Read","input":{"file_path":"main.go"}}`,
		`{"type":"tool_result","timestamp":"2026-04-24T10:00:00Z","tool_use_id":"tu1","output":"package main","is_error":false}`,
		`{"type":"system","timestamp":"2026-04-24T10:00:00Z","subtype":"compact_boundary","compact_metadata":{"trigger":"auto","pre_token_count":1000,"post_token_count":120}}`,
		`{"type":"summary","timestamp":"2026-04-24T10:00:00Z","summary":"session summary"}`,
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "transcript.jsonl")
	if err := os.WriteFile(path, []byte(joinLines(lines)), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	reader, err := NewReader(path)
	if err != nil {
		t.Fatalf("new reader: %v", err)
	}
	defer reader.Close()

	// user
	entry, err := reader.ReadNext()
	if err != nil {
		t.Fatalf("read user: %v", err)
	}
	ue, ok := entry.(UserEntry)
	if !ok {
		t.Fatalf("expected UserEntry, got %T", entry)
	}
	if ue.Type != "user" {
		t.Fatalf("type = %q, want user", ue.Type)
	}
	if len(ue.Message.Content) != 1 || ue.Message.Content[0].Text != "hello" {
		t.Fatalf("user message content mismatch: %+v", ue.Message.Content)
	}

	// assistant
	entry, err = reader.ReadNext()
	if err != nil {
		t.Fatalf("read assistant: %v", err)
	}
	ae, ok := entry.(AssistantEntry)
	if !ok {
		t.Fatalf("expected AssistantEntry, got %T", entry)
	}
	if ae.Message.Content[0].Text != "hi there" {
		t.Fatalf("assistant text mismatch")
	}

	// tool_use
	entry, err = reader.ReadNext()
	if err != nil {
		t.Fatalf("read tool_use: %v", err)
	}
	te, ok := entry.(ToolUseEntry)
	if !ok {
		t.Fatalf("expected ToolUseEntry, got %T", entry)
	}
	if te.Name != "Read" {
		t.Fatalf("tool name = %q, want Read", te.Name)
	}

	// tool_result
	entry, err = reader.ReadNext()
	if err != nil {
		t.Fatalf("read tool_result: %v", err)
	}
	re, ok := entry.(ToolResultEntry)
	if !ok {
		t.Fatalf("expected ToolResultEntry, got %T", entry)
	}
	if re.Output != "package main" {
		t.Fatalf("tool result output mismatch")
	}

	// system
	entry, err = reader.ReadNext()
	if err != nil {
		t.Fatalf("read system: %v", err)
	}
	se, ok := entry.(SystemEntry)
	if !ok {
		t.Fatalf("expected SystemEntry, got %T", entry)
	}
	if se.Subtype != "compact_boundary" {
		t.Fatalf("subtype = %q, want compact_boundary", se.Subtype)
	}

	// summary
	entry, err = reader.ReadNext()
	if err != nil {
		t.Fatalf("read summary: %v", err)
	}
	sue, ok := entry.(SummaryEntry)
	if !ok {
		t.Fatalf("expected SummaryEntry, got %T", entry)
	}
	if sue.Summary != "session summary" {
		t.Fatalf("summary mismatch")
	}

	// EOF
	_, err = reader.ReadNext()
	if err != io.EOF {
		t.Fatalf("expected EOF, got %v", err)
	}
}

func TestReader_ReadNext_SkipsMalformedLine(t *testing.T) {
	lines := []string{
		`{"type":"user","timestamp":"2026-04-24T10:00:00Z","message":{"role":"user","content":[]}}`,
		`this is not json`,
		`{"type":"assistant","timestamp":"2026-04-24T10:00:00Z","message":{"role":"assistant","content":[]}}`,
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "transcript.jsonl")
	if err := os.WriteFile(path, []byte(joinLines(lines)), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	reader, err := NewReader(path)
	if err != nil {
		t.Fatalf("new reader: %v", err)
	}
	defer reader.Close()

	entries, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("read all: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("entry count = %d, want 2", len(entries))
	}
	if _, ok := entries[0].(UserEntry); !ok {
		t.Fatalf("entries[0] type = %T, want UserEntry", entries[0])
	}
	if _, ok := entries[1].(AssistantEntry); !ok {
		t.Fatalf("entries[1] type = %T, want AssistantEntry", entries[1])
	}
}

func TestReader_ReadNext_SkipsUnknownType(t *testing.T) {
	lines := []string{
		`{"type":"user","timestamp":"2026-04-24T10:00:00Z","message":{"role":"user","content":[]}}`,
		`{"type":"future_entry","data":"unknown"}`,
		`{"type":"assistant","timestamp":"2026-04-24T10:00:00Z","message":{"role":"assistant","content":[]}}`,
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "transcript.jsonl")
	if err := os.WriteFile(path, []byte(joinLines(lines)), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	reader, err := NewReader(path)
	if err != nil {
		t.Fatalf("new reader: %v", err)
	}
	defer reader.Close()

	entries, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("read all: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("entry count = %d, want 2", len(entries))
	}
}

func TestReader_ReadNext_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.jsonl")
	if err := os.WriteFile(path, []byte{}, 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	reader, err := NewReader(path)
	if err != nil {
		t.Fatalf("new reader: %v", err)
	}
	defer reader.Close()

	entries, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("read all: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("entry count = %d, want 0", len(entries))
	}
}

func TestReader_ReadAll(t *testing.T) {
	lines := []string{
		`{"type":"user","timestamp":"2026-04-24T10:00:00Z","message":{"role":"user","content":[]}}`,
		`{"type":"assistant","timestamp":"2026-04-24T10:00:00Z","message":{"role":"assistant","content":[]}}`,
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "transcript.jsonl")
	if err := os.WriteFile(path, []byte(joinLines(lines)), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	reader, err := NewReader(path)
	if err != nil {
		t.Fatalf("new reader: %v", err)
	}
	defer reader.Close()

	entries, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("read all: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("entry count = %d, want 2", len(entries))
	}
}

func TestReader_ReadNext_WriterRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "roundtrip.jsonl")

	writer, err := NewWriter(path)
	if err != nil {
		t.Fatalf("new writer: %v", err)
	}

	ts := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	msg := message.Message{
		Role: message.RoleUser,
		Content: []message.ContentPart{
			message.TextPart("roundtrip test"),
		},
	}
	for _, entry := range EntriesFromMessage(ts, msg) {
		if err := writer.WriteEntry(entry); err != nil {
			t.Fatalf("write entry: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	reader, err := NewReader(path)
	if err != nil {
		t.Fatalf("new reader: %v", err)
	}
	defer reader.Close()

	entries, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("read all: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("entry count = %d, want 1 (UserEntry only, no tool results)", len(entries))
	}

	ue, ok := entries[0].(UserEntry)
	if !ok {
		t.Fatalf("entries[0] type = %T, want UserEntry", entries[0])
	}
	if ue.Message.Content[0].Text != "roundtrip test" {
		t.Fatalf("content = %q, want 'roundtrip test'", ue.Message.Content[0].Text)
	}
}

func joinLines(lines []string) string {
	var out []byte
	for i, line := range lines {
		if i > 0 {
			out = append(out, '\n')
		}
		out = append(out, []byte(line)...)
	}
	return string(out)
}
