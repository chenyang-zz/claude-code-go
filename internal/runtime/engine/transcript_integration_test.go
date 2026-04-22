package engine

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/conversation"
	"github.com/sheepzhao/claude-code-go/internal/core/message"
	"github.com/sheepzhao/claude-code-go/internal/core/model"
	"github.com/sheepzhao/claude-code-go/internal/core/tool"
	"github.com/sheepzhao/claude-code-go/internal/core/transcript"
)

func TestRuntimeRun_WritesTranscriptEntriesForToolLoop(t *testing.T) {
	transcriptPath := filepath.Join(t.TempDir(), "session.jsonl")
	client := &fakeModelClient{
		streams: []model.Stream{
			newModelStream(
				model.Event{
					Type: model.EventTypeToolUse,
					ToolUse: &model.ToolUse{
						ID:   "toolu_1",
						Name: "Read",
						Input: map[string]any{
							"file_path": "main.go",
						},
					},
				},
				model.Event{Type: model.EventTypeDone, StopReason: model.StopReasonToolUse},
			),
			newModelStream(
				model.Event{Type: model.EventTypeTextDelta, Text: "done"},
				model.Event{Type: model.EventTypeDone, StopReason: model.StopReasonEndTurn},
			),
		},
	}
	executor := &fakeToolExecutor{
		results: map[string]tool.Result{
			"Read": {Output: "file contents"},
		},
	}
	runtime := New(client, "claude-sonnet-4-5", executor)
	runtime.TranscriptPath = transcriptPath

	out, err := runtime.Run(context.Background(), conversation.RunRequest{
		SessionID: "session-1",
		Input:     "read the file",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	for range out {
	}

	lines := readTranscriptLines(t, transcriptPath)
	if len(lines) == 0 {
		t.Fatal("transcript line count = 0, want non-zero")
	}

	types := make([]string, 0, len(lines))
	for _, line := range lines {
		typeValue, _ := line["type"].(string)
		types = append(types, typeValue)
	}
	expected := []string{"user", "assistant", "tool_use", "user", "tool_result", "assistant"}
	if !equalStringSlices(types, expected) {
		t.Fatalf("transcript types = %#v, want %#v", types, expected)
	}
}

func TestRuntimeRun_WritesCompactSummaryAndBoundaryEntries(t *testing.T) {
	transcriptPath := filepath.Join(t.TempDir(), "session.jsonl")
	callCount := 0
	client := &fakeModelClient{
		streamFn: func(ctx context.Context, req model.Request) (model.Stream, error) {
			callCount++
			switch callCount {
			case 1:
				return nil, errors.New("prompt is too long: 250000 tokens > 200000")
			case 2:
				return newModelStream(
					model.Event{Type: model.EventTypeTextDelta, Text: "summary content"},
					model.Event{Type: model.EventTypeDone, StopReason: model.StopReasonEndTurn},
				), nil
			default:
				return newModelStream(
					model.Event{Type: model.EventTypeTextDelta, Text: "final answer"},
					model.Event{Type: model.EventTypeDone, StopReason: model.StopReasonEndTurn},
				), nil
			}
		},
	}
	runtime := New(client, "claude-sonnet-4-20250514", nil)
	runtime.AutoCompact = true
	runtime.TranscriptPath = transcriptPath

	out, err := runtime.Run(context.Background(), conversation.RunRequest{
		SessionID: "session-compact",
		Input:     "hello",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	for range out {
	}

	lines := readTranscriptLines(t, transcriptPath)
	if len(lines) == 0 {
		t.Fatal("transcript line count = 0, want non-zero")
	}

	var hasSummary bool
	var hasCompactBoundary bool
	for _, line := range lines {
		typeValue, _ := line["type"].(string)
		subtypeValue, _ := line["subtype"].(string)
		if typeValue == "summary" {
			hasSummary = true
		}
		if typeValue == "system" && subtypeValue == "compact_boundary" {
			hasCompactBoundary = true
		}
	}
	if !hasSummary {
		t.Fatal("missing summary transcript entry after compaction")
	}
	if !hasCompactBoundary {
		t.Fatal("missing compact_boundary transcript entry after compaction")
	}
}

func TestRuntimeRun_ResolvesTranscriptPathFromSessionAndCWD(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", configDir)

	client := &fakeModelClient{
		streams: []model.Stream{
			newModelStream(
				model.Event{Type: model.EventTypeTextDelta, Text: "ok"},
				model.Event{Type: model.EventTypeDone, StopReason: model.StopReasonEndTurn},
			),
		},
	}
	runtime := New(client, "claude-sonnet-4-5", nil)

	sessionID := "session-path"
	cwd := "/repo/path"
	out, err := runtime.Run(context.Background(), conversation.RunRequest{
		SessionID: sessionID,
		Input:     "hello",
		CWD:       cwd,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	for range out {
	}

	expectedPath := transcript.GetTranscriptPath(sessionID, cwd)
	if runtime.TranscriptPath != expectedPath {
		t.Fatalf("runtime transcript path = %q, want %q", runtime.TranscriptPath, expectedPath)
	}
	if _, err := os.Stat(expectedPath); err != nil {
		t.Fatalf("stat transcript path %q: %v", expectedPath, err)
	}
}

func readTranscriptLines(t *testing.T, path string) []map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read transcript file %q: %v", path, err)
	}
	text := strings.TrimSpace(string(data))
	if text == "" {
		return nil
	}

	rawLines := strings.Split(text, "\n")
	lines := make([]map[string]any, 0, len(rawLines))
	for _, rawLine := range rawLines {
		var item map[string]any
		if err := json.Unmarshal([]byte(rawLine), &item); err != nil {
			t.Fatalf("unmarshal transcript line %q: %v", rawLine, err)
		}
		lines = append(lines, item)
	}
	return lines
}

func equalStringSlices(got []string, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func TestRuntimeRun_TranscriptIncludesToolResultErrorFlag(t *testing.T) {
	transcriptPath := filepath.Join(t.TempDir(), "session.jsonl")
	client := &fakeModelClient{
		streams: []model.Stream{
			newModelStream(
				model.Event{
					Type:    model.EventTypeToolUse,
					ToolUse: &model.ToolUse{ID: "toolu_1", Name: "Edit"},
				},
				model.Event{Type: model.EventTypeDone, StopReason: model.StopReasonToolUse},
			),
			newModelStream(
				model.Event{Type: model.EventTypeTextDelta, Text: "handled"},
				model.Event{Type: model.EventTypeDone, StopReason: model.StopReasonEndTurn},
			),
		},
	}
	executor := &fakeToolExecutor{
		results: map[string]tool.Result{
			"Edit": {Error: "tool failed"},
		},
	}
	runtime := New(client, "claude-sonnet-4-5", executor)
	runtime.TranscriptPath = transcriptPath

	out, err := runtime.Run(context.Background(), conversation.RunRequest{
		SessionID: "session-tool-error",
		Messages: []message.Message{
			{
				Role: message.RoleUser,
				Content: []message.ContentPart{
					message.TextPart("edit the file"),
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	for range out {
	}

	lines := readTranscriptLines(t, transcriptPath)
	for _, line := range lines {
		typeValue, _ := line["type"].(string)
		if typeValue != "tool_result" {
			continue
		}
		isError, ok := line["is_error"].(bool)
		if !ok {
			t.Fatalf("tool_result is_error type = %T, want bool", line["is_error"])
		}
		if !isError {
			t.Fatal("tool_result is_error = false, want true for failed tool result")
		}
		return
	}

	t.Fatal("missing tool_result transcript entry")
}
