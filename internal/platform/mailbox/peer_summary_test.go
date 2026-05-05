package mailbox

import (
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
)

func TestGetLastPeerDmSummary_EmptyMessages(t *testing.T) {
	result := GetLastPeerDmSummary(nil)
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}

	result = GetLastPeerDmSummary([]message.Message{})
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}

func TestGetLastPeerDmSummary_NoToolUse(t *testing.T) {
	msgs := []message.Message{
		{Role: message.RoleUser, Content: []message.ContentPart{
			{Type: "text", Text: "hello"},
		}},
		{Role: message.RoleAssistant, Content: []message.ContentPart{
			{Type: "text", Text: "Hi there!"},
		}},
	}
	result := GetLastPeerDmSummary(msgs)
	if result != "" {
		t.Errorf("expected empty for no tool_use, got %q", result)
	}
}

func TestGetLastPeerDmSummary_StopsAtWakeUpBoundary(t *testing.T) {
	// Simulate a wake-up boundary: the last messages are from a previous turn
	msgs := []message.Message{
		{Role: message.RoleUser, Content: []message.ContentPart{
			{Type: "text", Text: "first turn"},
		}},
		{Role: message.RoleAssistant, Content: []message.ContentPart{
			{Type: "tool_use", ToolName: "SendMessage", ToolInput: map[string]any{
				"to":      "buddy",
				"message": "hello from first turn",
			}},
		}},
		{Role: message.RoleUser, Content: []message.ContentPart{
			{Type: "text", Text: "second turn (wake-up)"},
		}},
		{Role: message.RoleAssistant, Content: []message.ContentPart{
			{Type: "text", Text: "in second turn"},
		}},
	}
	// The SendMessage tool_use is before the wake-up boundary, should not be found
	result := GetLastPeerDmSummary(msgs)
	if result != "" {
		t.Errorf("expected empty (crosses wake-up boundary), got %q", result)
	}
}

func TestGetLastPeerDmSummary_SkipsBroadcast(t *testing.T) {
	msgs := []message.Message{
		{Role: message.RoleUser, Content: []message.ContentPart{
			{Type: "text", Text: "broadcast test"},
		}},
		{Role: message.RoleAssistant, Content: []message.ContentPart{
			{Type: "tool_use", ToolName: "SendMessage", ToolInput: map[string]any{
				"to":      "*",
				"message": "broadcast to all",
			}},
		}},
	}
	result := GetLastPeerDmSummary(msgs)
	if result != "" {
		t.Errorf("expected empty for broadcast, got %q", result)
	}
}

func TestGetLastPeerDmSummary_SkipsTeamLead(t *testing.T) {
	msgs := []message.Message{
		{Role: message.RoleUser, Content: []message.ContentPart{
			{Type: "text", Text: "team lead test"},
		}},
		{Role: message.RoleAssistant, Content: []message.ContentPart{
			{Type: "tool_use", ToolName: "SendMessage", ToolInput: map[string]any{
				"to":      "team-lead",
				"message": "report to lead",
			}},
		}},
	}
	result := GetLastPeerDmSummary(msgs)
	if result != "" {
		t.Errorf("expected empty for team-lead, got %q", result)
	}
}

func TestGetLastPeerDmSummary_UsesSummaryField(t *testing.T) {
	msgs := []message.Message{
		{Role: message.RoleUser, Content: []message.ContentPart{
			{Type: "text", Text: "peer dm test"},
		}},
		{Role: message.RoleAssistant, Content: []message.ContentPart{
			{Type: "tool_use", ToolName: "SendMessage", ToolInput: map[string]any{
				"to":      "worker-1",
				"message": "Please complete the database migration task",
				"summary": "db migration task",
			}},
		}},
	}
	result := GetLastPeerDmSummary(msgs)
	expected := "[to worker-1] db migration task"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestGetLastPeerDmSummary_FallbackToMessagePrefix(t *testing.T) {
	msgs := []message.Message{
		{Role: message.RoleUser, Content: []message.ContentPart{
			{Type: "text", Text: "no summary test"},
		}},
		{Role: message.RoleAssistant, Content: []message.ContentPart{
			{Type: "tool_use", ToolName: "SendMessage", ToolInput: map[string]any{
				"to":      "worker-1",
				"message": "short message",
			}},
		}},
	}
	result := GetLastPeerDmSummary(msgs)
	expected := "[to worker-1] short message"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestGetLastPeerDmSummary_TruncatesLongMessage(t *testing.T) {
	longMsg := ""
	for i := 0; i < 100; i++ {
		longMsg += "x"
	}
	msgs := []message.Message{
		{Role: message.RoleUser, Content: []message.ContentPart{
			{Type: "text", Text: "truncation test"},
		}},
		{Role: message.RoleAssistant, Content: []message.ContentPart{
			{Type: "tool_use", ToolName: "SendMessage", ToolInput: map[string]any{
				"to":      "worker-1",
				"message": longMsg,
			}},
		}},
	}
	result := GetLastPeerDmSummary(msgs)
	expected := "[to worker-1] " + longMsg[:80]
	if result != expected {
		t.Errorf("expected truncated result (len=%d), got len=%d", len(expected), len(result))
	}
}

func TestGetLastPeerDmSummary_SkipsDifferentTool(t *testing.T) {
	msgs := []message.Message{
		{Role: message.RoleUser, Content: []message.ContentPart{
			{Type: "text", Text: "other tool test"},
		}},
		{Role: message.RoleAssistant, Content: []message.ContentPart{
			{Type: "tool_use", ToolName: "Bash", ToolInput: map[string]any{
				"command": "ls -la",
			}},
		}},
	}
	result := GetLastPeerDmSummary(msgs)
	if result != "" {
		t.Errorf("expected empty for non-SendMessage tool, got %q", result)
	}
}

func TestGetLastPeerDmSummary_PicksLastPeerDmOnly(t *testing.T) {
	msgs := []message.Message{
		{Role: message.RoleUser, Content: []message.ContentPart{
			{Type: "text", Text: "multi dm test"},
		}},
		{Role: message.RoleAssistant, Content: []message.ContentPart{
			{Type: "tool_use", ToolName: "SendMessage", ToolInput: map[string]any{
				"to":      "worker-1",
				"message": "first task",
				"summary": "task 1",
			}},
			{Type: "tool_use", ToolName: "SendMessage", ToolInput: map[string]any{
				"to":      "worker-2",
				"message": "second task",
				"summary": "task 2",
			}},
		}},
	}
	// TS semantics: returns the FIRST matching SendMessage in the LAST assistant message
	result := GetLastPeerDmSummary(msgs)
	expected := "[to worker-1] task 1"
	if result != expected {
		t.Errorf("expected %q (first in last assistant msg), got %q", expected, result)
	}
}

func TestGetLastPeerDmSummary_SkipsUserToolResults(t *testing.T) {
	msgs := []message.Message{
		{Role: message.RoleUser, Content: []message.ContentPart{
			{Type: "tool_result", Text: "result data"},
		}},
		{Role: message.RoleAssistant, Content: []message.ContentPart{
			{Type: "tool_use", ToolName: "SendMessage", ToolInput: map[string]any{
				"to":      "worker-1",
				"message": "hello from assistant after tool result",
				"summary": "after tool result",
			}},
		}},
	}
	result := GetLastPeerDmSummary(msgs)
	expected := "[to worker-1] after tool result"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}
