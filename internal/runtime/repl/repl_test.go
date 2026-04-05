package repl

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/conversation"
	"github.com/sheepzhao/claude-code-go/internal/core/event"
	"github.com/sheepzhao/claude-code-go/internal/core/message"
	coresession "github.com/sheepzhao/claude-code-go/internal/core/session"
	"github.com/sheepzhao/claude-code-go/internal/runtime/engine"
	runtimesession "github.com/sheepzhao/claude-code-go/internal/runtime/session"
	"github.com/sheepzhao/claude-code-go/internal/ui/console"
)

type recordingEngine struct {
	lastRequest conversation.RunRequest
	stream      event.Stream
}

type recordingSessionRepository struct {
	loadResult   coresession.Session
	loadErr      error
	latestResult coresession.Session
	latestErr    error
	saved        []coresession.Session
}

func (e *recordingEngine) Run(ctx context.Context, req conversation.RunRequest) (event.Stream, error) {
	e.lastRequest = req
	return e.stream, nil
}

func (r *recordingSessionRepository) Save(ctx context.Context, session coresession.Session) error {
	_ = ctx
	r.saved = append(r.saved, session.Clone())
	return nil
}

func (r *recordingSessionRepository) Load(ctx context.Context, id string) (coresession.Session, error) {
	_ = ctx
	if r.loadErr != nil {
		return coresession.Session{}, r.loadErr
	}
	if r.loadResult.ID == "" && r.latestResult.ID == id {
		return r.latestResult.Clone(), nil
	}
	return r.loadResult.Clone(), nil
}

func (r *recordingSessionRepository) LoadLatest(ctx context.Context, lookup coresession.Lookup) (coresession.Session, error) {
	_ = ctx
	if r.latestErr != nil {
		return coresession.Session{}, r.latestErr
	}
	if r.latestResult.ID != "" || len(r.latestResult.Messages) != 0 || r.latestResult.ProjectPath != "" {
		if r.latestResult.ProjectPath != lookup.ProjectPath {
			return coresession.Session{}, coresession.ErrSessionNotFound
		}
		return r.latestResult.Clone(), nil
	}
	if r.loadErr != nil {
		return coresession.Session{}, r.loadErr
	}
	return r.loadResult.Clone(), nil
}

var _ engine.Engine = (*recordingEngine)(nil)

// TestParseArgsParsesSlashCommand verifies slash input is split into command and body.
func TestParseArgsParsesSlashCommand(t *testing.T) {
	parsed, err := ParseArgs([]string{"/help", "topic"})
	if err != nil {
		t.Fatalf("ParseArgs() error = %v", err)
	}

	if !parsed.IsSlashCommand || parsed.Command != "help" || parsed.Body != "topic" {
		t.Fatalf("ParseArgs() = %#v, want slash command help with body topic", parsed)
	}
}

// TestRunnerRunForwardsPrompt verifies the REPL forwards real CLI input into the engine request.
func TestRunnerRunForwardsPrompt(t *testing.T) {
	stream := make(chan event.Event, 2)
	stream <- event.Event{
		Type:      event.TypeMessageDelta,
		Timestamp: time.Now(),
		Payload: event.MessageDeltaPayload{
			Text: "hello",
		},
	}
	stream <- event.Event{
		Type:      event.TypeConversationDone,
		Timestamp: time.Now(),
		Payload: event.ConversationDonePayload{
			History: conversation.History{
				Messages: []message.Message{
					{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("say hello")}},
					{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("hello")}},
				},
			},
		},
	}
	close(stream)

	var buf bytes.Buffer
	eng := &recordingEngine{stream: stream}
	runner := NewRunner(eng, console.NewStreamRenderer(console.NewPrinter(&buf)))

	if err := runner.Run(context.Background(), []string{"say", "hello"}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if eng.lastRequest.Input != "" {
		t.Fatalf("Run() request.Input = %q, want empty input because REPL sends full message history", eng.lastRequest.Input)
	}
	if len(eng.lastRequest.Messages) != 1 || eng.lastRequest.Messages[0].Content[0].Text != "say hello" {
		t.Fatalf("Run() request.Messages = %#v, want one user message with say hello", eng.lastRequest.Messages)
	}

	if got := buf.String(); got != "hello" {
		t.Fatalf("Run() output = %q, want %q", got, "hello")
	}
}

// TestRunnerRunHandlesSlashPlaceholder verifies slash commands bypass the engine and render a stable placeholder.
func TestRunnerRunHandlesSlashPlaceholder(t *testing.T) {
	var buf bytes.Buffer
	eng := &recordingEngine{}
	runner := NewRunner(eng, console.NewStreamRenderer(console.NewPrinter(&buf)))

	if err := runner.Run(context.Background(), []string{"/help"}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if eng.lastRequest.Input != "" {
		t.Fatalf("engine should not be called for slash command, got request %#v", eng.lastRequest)
	}

	if got := buf.String(); got != "Slash command /help is not supported yet.\n" {
		t.Fatalf("Run() output = %q, want slash placeholder", got)
	}
}

// TestRunnerRunResumeRestoresSessionAndRunsPrompt verifies /resume restores an existing session and continues it with the trailing prompt text.
func TestRunnerRunResumeRestoresSessionAndRunsPrompt(t *testing.T) {
	stream := make(chan event.Event, 2)
	stream <- event.Event{
		Type:      event.TypeMessageDelta,
		Timestamp: time.Now(),
		Payload: event.MessageDeltaPayload{
			Text: "continued reply",
		},
	}
	stream <- event.Event{
		Type:      event.TypeConversationDone,
		Timestamp: time.Now(),
		Payload: event.ConversationDonePayload{
			History: conversation.History{
				Messages: []message.Message{
					{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("old prompt")}},
					{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("old reply")}},
					{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("new prompt")}},
					{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("continued reply")}},
				},
			},
		},
	}
	close(stream)

	repo := &recordingSessionRepository{
		loadResult: coresession.Session{
			ID: "session-2",
			Messages: []message.Message{
				{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("old prompt")}},
				{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("old reply")}},
			},
		},
	}
	manager := runtimesession.NewManager(repo)
	autosave := runtimesession.NewAutoSave(manager)

	var buf bytes.Buffer
	eng := &recordingEngine{stream: stream}
	runner := NewRunner(eng, console.NewStreamRenderer(console.NewPrinter(&buf)))
	runner.SessionManager = manager
	runner.AutoSave = autosave

	if err := runner.Run(context.Background(), []string{"/resume", "session-2", "new", "prompt"}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if eng.lastRequest.SessionID != "session-2" {
		t.Fatalf("Run() request.SessionID = %q, want session-2", eng.lastRequest.SessionID)
	}
	if len(eng.lastRequest.Messages) != 3 {
		t.Fatalf("Run() request message count = %d, want 3", len(eng.lastRequest.Messages))
	}
	if eng.lastRequest.Messages[2].Content[0].Text != "new prompt" {
		t.Fatalf("Run() request last message = %#v, want new prompt", eng.lastRequest.Messages[2])
	}
	if len(repo.saved) != 1 {
		t.Fatalf("autosave saved count = %d, want 1", len(repo.saved))
	}
	if repo.saved[0].ID != "session-2" {
		t.Fatalf("autosave saved session id = %q, want session-2", repo.saved[0].ID)
	}
	if got := buf.String(); got != "continued reply" {
		t.Fatalf("Run() output = %q, want %q", got, "continued reply")
	}
}

// TestRunnerRunResumeSessionNotFound verifies /resume reports a stable error when the target session does not exist.
func TestRunnerRunResumeSessionNotFound(t *testing.T) {
	var buf bytes.Buffer
	eng := &recordingEngine{}
	manager := runtimesession.NewManager(&recordingSessionRepository{loadErr: coresession.ErrSessionNotFound})
	runner := NewRunner(eng, console.NewStreamRenderer(console.NewPrinter(&buf)))
	runner.SessionManager = manager

	if err := runner.Run(context.Background(), []string{"/resume", "missing-session", "hello"}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if eng.lastRequest.SessionID != "" || len(eng.lastRequest.Messages) != 0 {
		t.Fatalf("engine should not be called for missing session, got request %#v", eng.lastRequest)
	}
	if got := buf.String(); got != "Session missing-session was not found.\n" {
		t.Fatalf("Run() output = %q, want stable missing-session error", got)
	}
}

// TestRunnerRunResumeRequiresPrompt verifies /resume rejects missing follow-up prompt text with a stable usage hint.
func TestRunnerRunResumeRequiresPrompt(t *testing.T) {
	var buf bytes.Buffer
	eng := &recordingEngine{}
	runner := NewRunner(eng, console.NewStreamRenderer(console.NewPrinter(&buf)))

	if err := runner.Run(context.Background(), []string{"/resume", "session-3"}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if eng.lastRequest.SessionID != "" || len(eng.lastRequest.Messages) != 0 {
		t.Fatalf("engine should not be called for invalid resume input, got request %#v", eng.lastRequest)
	}
	if got := buf.String(); got != resumeUsageMessage+"\n" {
		t.Fatalf("Run() output = %q, want resume usage hint", got)
	}
}

// TestRunnerRunRestoresAndAutosavesHistory verifies the REPL bridges persisted session history into the engine and saves the completed history afterwards.
func TestRunnerRunRestoresAndAutosavesHistory(t *testing.T) {
	stream := make(chan event.Event, 2)
	stream <- event.Event{
		Type:      event.TypeMessageDelta,
		Timestamp: time.Now(),
		Payload: event.MessageDeltaPayload{
			Text: "new reply",
		},
	}
	stream <- event.Event{
		Type:      event.TypeConversationDone,
		Timestamp: time.Now(),
		Payload: event.ConversationDonePayload{
			History: conversation.History{
				Messages: []message.Message{
					{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("old prompt")}},
					{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("old reply")}},
					{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("new prompt")}},
					{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("new reply")}},
				},
			},
		},
	}
	close(stream)

	repo := &recordingSessionRepository{
		loadResult: coresession.Session{
			ID: "session-1",
			Messages: []message.Message{
				{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("old prompt")}},
				{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("old reply")}},
			},
		},
	}
	manager := runtimesession.NewManager(repo)
	autosave := runtimesession.NewAutoSave(manager)

	var buf bytes.Buffer
	eng := &recordingEngine{stream: stream}
	runner := NewRunner(eng, console.NewStreamRenderer(console.NewPrinter(&buf)))
	runner.SessionID = "session-1"
	runner.SessionManager = manager
	runner.AutoSave = autosave

	if err := runner.Run(context.Background(), []string{"new", "prompt"}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if len(eng.lastRequest.Messages) != 3 {
		t.Fatalf("Run() request message count = %d, want 3", len(eng.lastRequest.Messages))
	}
	if eng.lastRequest.Messages[2].Content[0].Text != "new prompt" {
		t.Fatalf("Run() request last message = %#v, want new prompt", eng.lastRequest.Messages[2])
	}
	if len(repo.saved) != 1 {
		t.Fatalf("autosave saved count = %d, want 1", len(repo.saved))
	}
	if len(repo.saved[0].Messages) != 4 {
		t.Fatalf("autosave saved message count = %d, want 4", len(repo.saved[0].Messages))
	}
}

// TestRunnerRunResumesLatestSessionForProject verifies normal prompt execution restores the latest session in the current project.
func TestRunnerRunResumesLatestSessionForProject(t *testing.T) {
	stream := make(chan event.Event, 2)
	stream <- event.Event{
		Type:      event.TypeMessageDelta,
		Timestamp: time.Now(),
		Payload: event.MessageDeltaPayload{
			Text: "latest reply",
		},
	}
	stream <- event.Event{
		Type:      event.TypeConversationDone,
		Timestamp: time.Now(),
		Payload: event.ConversationDonePayload{
			History: conversation.History{
				Messages: []message.Message{
					{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("old prompt")}},
					{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("old reply")}},
					{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("follow-up")}},
					{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("latest reply")}},
				},
			},
		},
	}
	close(stream)

	repo := &recordingSessionRepository{
		latestResult: coresession.Session{
			ID:          "session-latest",
			ProjectPath: "/repo",
			Messages: []message.Message{
				{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("old prompt")}},
				{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("old reply")}},
			},
		},
	}
	manager := runtimesession.NewManager(repo)
	autosave := runtimesession.NewAutoSave(manager)

	var buf bytes.Buffer
	eng := &recordingEngine{stream: stream}
	runner := NewRunner(eng, console.NewStreamRenderer(console.NewPrinter(&buf)))
	runner.ProjectPath = "/repo"
	runner.SessionManager = manager
	runner.AutoSave = autosave

	if err := runner.Run(context.Background(), []string{"follow-up"}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if eng.lastRequest.SessionID != "session-latest" {
		t.Fatalf("Run() request.SessionID = %q, want session-latest", eng.lastRequest.SessionID)
	}
	if len(eng.lastRequest.Messages) != 3 {
		t.Fatalf("Run() request message count = %d, want 3", len(eng.lastRequest.Messages))
	}
	if len(repo.saved) != 1 {
		t.Fatalf("autosave saved count = %d, want 1", len(repo.saved))
	}
	if repo.saved[0].ID != "session-latest" {
		t.Fatalf("autosave saved session id = %q, want session-latest", repo.saved[0].ID)
	}
	if repo.saved[0].ProjectPath != "/repo" {
		t.Fatalf("autosave saved project path = %q, want /repo", repo.saved[0].ProjectPath)
	}
}

// TestRunnerRunStartsFreshProjectScopedSession verifies a missing latest session falls back to a new project-scoped session.
func TestRunnerRunStartsFreshProjectScopedSession(t *testing.T) {
	stream := make(chan event.Event, 2)
	stream <- event.Event{
		Type:      event.TypeMessageDelta,
		Timestamp: time.Now(),
		Payload: event.MessageDeltaPayload{
			Text: "fresh reply",
		},
	}
	stream <- event.Event{
		Type:      event.TypeConversationDone,
		Timestamp: time.Now(),
		Payload: event.ConversationDonePayload{
			History: conversation.History{
				Messages: []message.Message{
					{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("fresh prompt")}},
					{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("fresh reply")}},
				},
			},
		},
	}
	close(stream)

	repo := &recordingSessionRepository{latestErr: coresession.ErrSessionNotFound}
	manager := runtimesession.NewManager(repo)
	autosave := runtimesession.NewAutoSave(manager)

	var buf bytes.Buffer
	eng := &recordingEngine{stream: stream}
	runner := NewRunner(eng, console.NewStreamRenderer(console.NewPrinter(&buf)))
	runner.ProjectPath = "/repo"
	runner.SessionManager = manager
	runner.AutoSave = autosave

	if err := runner.Run(context.Background(), []string{"fresh", "prompt"}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if eng.lastRequest.SessionID == "" {
		t.Fatal("Run() request.SessionID = empty, want generated session id")
	}
	if len(repo.saved) != 1 {
		t.Fatalf("autosave saved count = %d, want 1", len(repo.saved))
	}
	if repo.saved[0].ProjectPath != "/repo" {
		t.Fatalf("autosave saved project path = %q, want /repo", repo.saved[0].ProjectPath)
	}
}
