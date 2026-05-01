package engine

import (
	"context"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/event"
	"github.com/sheepzhao/claude-code-go/internal/core/model"
	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
)

func TestToolResultImage(t *testing.T) {
	cases := []struct {
		name   string
		meta   map[string]any
		want   *coretool.ImageData
	}{
		{
			name: "has image",
			meta: map[string]any{
				"image": coretool.ImageData{MediaType: "image/jpeg", Base64: "abc"},
			},
			want: &coretool.ImageData{MediaType: "image/jpeg", Base64: "abc"},
		},
		{
			name: "no meta",
			meta: nil,
			want: nil,
		},
		{
			name: "no image key",
			meta: map[string]any{"data": "something"},
			want: nil,
		},
		{
			name: "wrong type",
			meta: map[string]any{"image": "not a struct"},
			want: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := coretool.Result{Meta: tc.meta}
			got := toolResultImage(result)
			if tc.want == nil {
				if got != nil {
					t.Fatalf("want nil, got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("want %+v, got nil", tc.want)
			}
			if got.MediaType != tc.want.MediaType || got.Base64 != tc.want.Base64 {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestStreamingToolExecutor_BuildToolResultMessage_WithImage(t *testing.T) {
	fake := newFakeStreamingExecute()
	fake.results["ToolA"] = coretool.Result{
		Output: "Read image (1.2 MB)",
		Meta: map[string]any{
			"image": coretool.ImageData{MediaType: "image/jpeg", Base64: "base64img"},
		},
	}
	fake.results["ToolB"] = coretool.Result{Output: "result-b"}

	out := make(chan event.Event, 10)
	exec := NewStreamingToolExecutor(
		context.Background(),
		fake.execute,
		func(string) bool { return true },
		out,
		10,
	)

	exec.AddTool(context.Background(), model.ToolUse{ID: "a", Name: "ToolA"})
	exec.AddTool(context.Background(), model.ToolUse{ID: "b", Name: "ToolB"})
	exec.AwaitAll(context.Background())

	msg := exec.BuildToolResultMessage()
	if msg.Role != "user" {
		t.Errorf("expected role 'user', got %s", msg.Role)
	}
	if len(msg.Content) != 3 {
		t.Fatalf("expected 3 content parts (tool_result A + image + tool_result B), got %d", len(msg.Content))
	}

	// First: tool_result for ToolA.
	if msg.Content[0].Type != "tool_result" || msg.Content[0].ToolUseID != "a" {
		t.Fatalf("first part = %+v, want tool_result for a", msg.Content[0])
	}

	// Second: image part appended after ToolA result.
	if msg.Content[1].Type != "image" || msg.Content[1].MediaType != "image/jpeg" || msg.Content[1].Base64Data != "base64img" {
		t.Fatalf("second part = %+v, want image", msg.Content[1])
	}

	// Third: tool_result for ToolB.
	if msg.Content[2].Type != "tool_result" || msg.Content[2].ToolUseID != "b" {
		t.Fatalf("third part = %+v, want tool_result for b", msg.Content[2])
	}
}

func TestStreamingToolExecutor_BuildToolResultMessage_ImageSkippedOnError(t *testing.T) {
	fake := newFakeStreamingExecute()
	fake.results["ToolA"] = coretool.Result{
		Error: "something went wrong",
		Meta: map[string]any{
			"image": coretool.ImageData{MediaType: "image/jpeg", Base64: "base64img"},
		},
	}

	out := make(chan event.Event, 10)
	exec := NewStreamingToolExecutor(
		context.Background(),
		fake.execute,
		func(string) bool { return true },
		out,
		10,
	)

	exec.AddTool(context.Background(), model.ToolUse{ID: "a", Name: "ToolA"})
	exec.AwaitAll(context.Background())

	msg := exec.BuildToolResultMessage()
	if len(msg.Content) != 1 {
		t.Fatalf("expected 1 content part (tool_result only, no image on error), got %d", len(msg.Content))
	}
	if msg.Content[0].Type != "tool_result" {
		t.Fatalf("part = %+v, want tool_result", msg.Content[0])
	}
}
