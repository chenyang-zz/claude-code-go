package remote

import (
	"testing"
)

func TestClassifyEvent(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		want    MessageCategory
		wantErr bool
	}{
		{
			name:    "control_request",
			payload: `{"type":"control_request","request_id":"r1","request":{"subtype":"can_use_tool"}}`,
			want:    CategoryControlRequest,
		},
		{
			name:    "control_response",
			payload: `{"type":"control_response","response":{"subtype":"success","request_id":"r1"}}`,
			want:    CategoryControlResponse,
		},
		{
			name:    "control_cancel_request",
			payload: `{"type":"control_cancel_request","request_id":"r1"}`,
			want:    CategoryControlCancelRequest,
		},
		{
			name:    "assistant",
			payload: `{"type":"assistant","message":{"role":"assistant"}}`,
			want:    CategorySDKMessage,
		},
		{
			name:    "user",
			payload: `{"type":"user","message":{"role":"user"}}`,
			want:    CategorySDKMessage,
		},
		{
			name:    "stream_event",
			payload: `{"type":"stream_event","event":{"type":"text_delta"}}`,
			want:    CategorySDKMessage,
		},
		{
			name:    "result",
			payload: `{"type":"result","subtype":"success"}`,
			want:    CategorySDKMessage,
		},
		{
			name:    "system",
			payload: `{"type":"system","subtype":"init"}`,
			want:    CategorySDKMessage,
		},
		{
			name:    "tool_progress",
			payload: `{"type":"tool_progress","tool_name":"Bash"}`,
			want:    CategorySDKMessage,
		},
		{
			name:    "auth_status",
			payload: `{"type":"auth_status"}`,
			want:    CategorySDKMessage,
		},
		{
			name:    "tool_use_summary",
			payload: `{"type":"tool_use_summary"}`,
			want:    CategorySDKMessage,
		},
		{
			name:    "rate_limit_event",
			payload: `{"type":"rate_limit_event"}`,
			want:    CategorySDKMessage,
		},
		{
			name:    "unknown_type",
			payload: `{"type":"future_message","data":123}`,
			want:    CategorySDKMessage,
		},
		{
			name:    "keep_alive",
			payload: `{"type":"keep_alive"}`,
			want:    CategorySDKMessage,
		},
		{
			name:    "invalid_json",
			payload: `not json`,
			wantErr: true,
		},
		{
			name:    "empty_object",
			payload: `{}`,
			want:    CategorySDKMessage,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ClassifyEvent([]byte(tt.payload))
			if (err != nil) != tt.wantErr {
				t.Fatalf("ClassifyEvent() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if got != tt.want {
				t.Errorf("ClassifyEvent() = %v, want %v", got, tt.want)
			}
		})
	}
}
