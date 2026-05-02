package promptsuggestion

import (
	"context"
	"os"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
)

func TestNewSpeculator(t *testing.T) {
	s := NewSpeculator()
	if s.State() != SpeculationStateIdle {
		t.Fatalf("expected state idle, got %s", s.State())
	}
}

func TestSpeculator_Start(t *testing.T) {
	os.Setenv("CLAUDE_FEATURE_SPECULATION", "1")
	defer os.Unsetenv("CLAUDE_FEATURE_SPECULATION")

	s := NewSpeculator()
	s.Start(context.Background(), StartParams{
		SuggestionText: "test suggestion",
		Messages:       []message.Message{},
	})

	if s.State() != SpeculationStateActive {
		t.Fatalf("expected state active, got %s", s.State())
	}

	s.Abort()
}

func TestSpeculator_Abort(t *testing.T) {
	os.Setenv("CLAUDE_FEATURE_SPECULATION", "1")
	defer os.Unsetenv("CLAUDE_FEATURE_SPECULATION")

	s := NewSpeculator()
	s.Start(context.Background(), StartParams{
		SuggestionText: "test suggestion",
		Messages:       []message.Message{},
	})

	if s.State() != SpeculationStateActive {
		t.Fatalf("expected state active before abort, got %s", s.State())
	}

	s.Abort()

	if s.State() != SpeculationStateIdle {
		t.Fatalf("expected state idle after abort, got %s", s.State())
	}
}

func TestSpeculator_StartWhileActive(t *testing.T) {
	os.Setenv("CLAUDE_FEATURE_SPECULATION", "1")
	defer os.Unsetenv("CLAUDE_FEATURE_SPECULATION")

	s := NewSpeculator()

	// 第一次启动
	s.Start(context.Background(), StartParams{
		SuggestionText: "first suggestion",
		Messages:       []message.Message{},
	})

	if s.State() != SpeculationStateActive {
		t.Fatalf("expected state active after first start, got %s", s.State())
	}

	oldActive := s.active

	// 第二次启动（应在内部先 abort 旧的）
	s.Start(context.Background(), StartParams{
		SuggestionText: "second suggestion",
		Messages:       []message.Message{},
	})

	if s.State() != SpeculationStateActive {
		t.Fatalf("expected state active after second start, got %s", s.State())
	}

	if s.active == oldActive {
		t.Fatal("expected new active speculation state after second start")
	}

	s.Abort()
}

func TestSpeculator_Start_Disabled(t *testing.T) {
	os.Setenv("CLAUDE_FEATURE_SPECULATION", "0")
	defer os.Unsetenv("CLAUDE_FEATURE_SPECULATION")

	s := NewSpeculator()
	s.Start(context.Background(), StartParams{
		SuggestionText: "test suggestion",
		Messages:       []message.Message{},
	})

	if s.State() != SpeculationStateIdle {
		t.Fatalf("expected state idle when disabled, got %s", s.State())
	}
}

func TestPrepareMessagesForInjection(t *testing.T) {
	messages := []message.Message{
		{
			Role: message.RoleUser,
			Content: []message.ContentPart{
				message.TextPart("hello"),
				message.ThinkingPart("thinking content", "sig"),
				message.TextPart("world"),
			},
		},
		{
			Role: message.RoleAssistant,
			Content: []message.ContentPart{
				message.RedactedThinkingPart("redacted data"),
			},
		},
		{
			Role: message.RoleUser,
			Content: []message.ContentPart{
				message.TextPart("   "),
				message.TextPart("  \n\t  "),
			},
		},
		{
			Role: message.RoleUser,
			Content: []message.ContentPart{
				message.TextPart("keep this"),
			},
		},
	}

	result := prepareMessagesForInjection(messages)

	if len(result) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(result))
	}

	// 第一条消息：过滤 thinking，保留 hello 和 world
	if len(result[0].Content) != 2 {
		t.Fatalf("expected 2 content parts in first message, got %d", len(result[0].Content))
	}
	if result[0].Content[0].Text != "hello" {
		t.Fatalf("expected first part text 'hello', got %s", result[0].Content[0].Text)
	}
	if result[0].Content[1].Text != "world" {
		t.Fatalf("expected second part text 'world', got %s", result[0].Content[1].Text)
	}

	// 第二条消息：纯空白文本被过滤，保留 keep this
	if result[1].Content[0].Text != "keep this" {
		t.Fatalf("expected 'keep this', got %s", result[1].Content[0].Text)
	}
}
