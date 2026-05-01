package message

import (
	"encoding/json"
	"testing"
)

func TestImagePart(t *testing.T) {
	part := ImagePart("image/jpeg", "base64data")
	if part.Type != "image" {
		t.Errorf("Type = %q, want %q", part.Type, "image")
	}
	if part.MediaType != "image/jpeg" {
		t.Errorf("MediaType = %q, want %q", part.MediaType, "image/jpeg")
	}
	if part.Base64Data != "base64data" {
		t.Errorf("Base64Data = %q, want %q", part.Base64Data, "base64data")
	}
}

func TestContentPartSerialization(t *testing.T) {
	cases := []struct {
		name string
		part ContentPart
		want string
	}{
		{
			name: "text",
			part: TextPart("hello"),
			want: `{"type":"text","text":"hello"}`,
		},
		{
			name: "tool_use",
			part: ToolUsePart("id1", "Read", map[string]any{"file_path": "/tmp/a"}),
			want: `{"type":"tool_use","tool_use_id":"id1","tool_name":"Read","tool_input":{"file_path":"/tmp/a"}}`,
		},
		{
			name: "tool_result",
			part: ToolResultPart("id1", "result", false),
			want: `{"type":"tool_result","text":"result","tool_use_id":"id1"}`,
		},
		{
			name: "image",
			part: ImagePart("image/png", "dGVzdA=="),
			want: `{"type":"image","media_type":"image/png","base64_data":"dGVzdA=="}`,
		},
		{
			name: "thinking",
			part: ThinkingPart("think", "sig"),
			want: `{"type":"thinking","thinking":"think","signature":"sig"}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := json.Marshal(tc.part)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if string(got) != tc.want {
				t.Errorf("got %s, want %s", got, tc.want)
			}
		})
	}
}

func TestContentPartDeserialization(t *testing.T) {
	cases := []struct {
		name string
		json string
		want ContentPart
	}{
		{
			name: "image",
			json: `{"type":"image","media_type":"image/jpeg","base64_data":"abc"}`,
			want: ImagePart("image/jpeg", "abc"),
		},
		{
			name: "text",
			json: `{"type":"text","text":"hello"}`,
			want: TextPart("hello"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got ContentPart
			if err := json.Unmarshal([]byte(tc.json), &got); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if got.Type != tc.want.Type {
				t.Errorf("Type = %q, want %q", got.Type, tc.want.Type)
			}
			if got.MediaType != tc.want.MediaType {
				t.Errorf("MediaType = %q, want %q", got.MediaType, tc.want.MediaType)
			}
			if got.Base64Data != tc.want.Base64Data {
				t.Errorf("Base64Data = %q, want %q", got.Base64Data, tc.want.Base64Data)
			}
		})
	}
}
