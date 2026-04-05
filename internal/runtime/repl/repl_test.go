package repl

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/internal/core/conversation"
	"github.com/sheepzhao/claude-code-go/internal/core/event"
	"github.com/sheepzhao/claude-code-go/internal/core/message"
	coresession "github.com/sheepzhao/claude-code-go/internal/core/session"
	"github.com/sheepzhao/claude-code-go/internal/runtime/engine"
	runtimesession "github.com/sheepzhao/claude-code-go/internal/runtime/session"
	servicecommands "github.com/sheepzhao/claude-code-go/internal/services/commands"
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
	listRecent   []coresession.Summary
	listErr      error
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
	for i := len(r.saved) - 1; i >= 0; i-- {
		if r.saved[i].ID == id {
			return r.saved[i].Clone(), nil
		}
	}
	if r.latestResult.ID == id {
		return r.latestResult.Clone(), nil
	}
	if r.loadResult.ID == id {
		return r.loadResult.Clone(), nil
	}
	return coresession.Session{}, coresession.ErrSessionNotFound
}

func (r *recordingSessionRepository) LoadLatest(ctx context.Context, lookup coresession.Lookup) (coresession.Session, error) {
	_ = ctx
	if r.latestErr != nil {
		return coresession.Session{}, r.latestErr
	}
	for i := len(r.saved) - 1; i >= 0; i-- {
		if r.saved[i].ProjectPath == lookup.ProjectPath {
			return r.saved[i].Clone(), nil
		}
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

func (r *recordingSessionRepository) ListRecent(ctx context.Context, lookup coresession.Lookup) ([]coresession.Summary, error) {
	_ = ctx
	if r.listErr != nil {
		return nil, r.listErr
	}
	var filtered []coresession.Summary
	for _, summary := range r.listRecent {
		if summary.ProjectPath == lookup.ProjectPath {
			filtered = append(filtered, summary)
		}
		if lookup.Limit > 0 && len(filtered) == lookup.Limit {
			break
		}
	}
	return filtered, nil
}

var _ engine.Engine = (*recordingEngine)(nil)

type staticCommand struct {
	meta   command.Metadata
	result command.Result
}

func (c staticCommand) Metadata() command.Metadata {
	return c.meta
}

func (c staticCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx
	_ = args
	return c.result, nil
}

func registerSlashCommands(t *testing.T, runner *Runner, commands ...command.Command) {
	t.Helper()

	registry := command.NewInMemoryRegistry()
	for _, cmd := range commands {
		if err := registry.Register(cmd); err != nil {
			t.Fatalf("Register(%q) error = %v", cmd.Metadata().Name, err)
		}
	}
	runner.Commands = registry
}

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

// TestParseArgsParsesContinueFlags verifies the minimum continue/fork flags are peeled off before prompt parsing.
func TestParseArgsParsesContinueFlags(t *testing.T) {
	parsed, err := ParseArgs([]string{"--fork-session", "--continue", "follow", "up"})
	if err != nil {
		t.Fatalf("ParseArgs() error = %v", err)
	}

	if !parsed.ContinueLatest {
		t.Fatal("ParseArgs() ContinueLatest = false, want true")
	}
	if !parsed.ForkSession {
		t.Fatal("ParseArgs() ForkSession = false, want true")
	}
	if parsed.Body != "follow up" {
		t.Fatalf("ParseArgs() body = %q, want %q", parsed.Body, "follow up")
	}
}

// TestParseArgsRejectsForkWithoutExplicitRecovery verifies fork mode is only accepted for continue/resume recovery flows.
func TestParseArgsRejectsForkWithoutExplicitRecovery(t *testing.T) {
	_, err := ParseArgs([]string{"--fork-session", "follow", "up"})
	if err == nil {
		t.Fatal("ParseArgs() error = nil, want fork usage error")
	}
	if err.Error() != forkUsageMessage {
		t.Fatalf("ParseArgs() error = %q, want %q", err.Error(), forkUsageMessage)
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

// TestRunnerRunDispatchesRegisteredSlashCommand verifies slash commands are dispatched through the shared registry.
func TestRunnerRunDispatchesRegisteredSlashCommand(t *testing.T) {
	var buf bytes.Buffer
	eng := &recordingEngine{}
	runner := NewRunner(eng, console.NewStreamRenderer(console.NewPrinter(&buf)))
	registerSlashCommands(t, runner, staticCommand{
		meta: command.Metadata{
			Name:        "help",
			Description: "show help",
			Usage:       "/help",
		},
		result: command.Result{Output: "available commands"},
	})

	if err := runner.Run(context.Background(), []string{"/help"}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if eng.lastRequest.Input != "" {
		t.Fatalf("engine should not be called for slash command, got request %#v", eng.lastRequest)
	}

	if got := buf.String(); got != "available commands\n" {
		t.Fatalf("Run() output = %q, want registered slash command output", got)
	}
}

// TestRunnerRunHelpCommandListsRegisteredCommands verifies the real /help command output matches the registry-backed support surface.
func TestRunnerRunHelpCommandListsRegisteredCommands(t *testing.T) {
	var buf bytes.Buffer
	eng := &recordingEngine{}
	runner := NewRunner(eng, console.NewStreamRenderer(console.NewPrinter(&buf)))

	registry := command.NewInMemoryRegistry()
	if err := registry.Register(servicecommands.HelpCommand{Registry: registry}); err != nil {
		t.Fatalf("Register(help) error = %v", err)
	}
	if err := registry.Register(servicecommands.ClearCommand{}); err != nil {
		t.Fatalf("Register(clear) error = %v", err)
	}
	if err := registry.Register(NewResumeCommandAdapter(runner)); err != nil {
		t.Fatalf("Register(resume) error = %v", err)
	}
	runner.Commands = registry

	if err := runner.Run(context.Background(), []string{"/help"}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	want := "Available commands:\n/help - Show help and available commands\n/clear - Clear conversation history and start a new session\n/resume - Resume a saved session and continue it with a new prompt\n  Aliases: /continue\n  Usage: /resume <session-id> <prompt>\nSend plain text without a leading slash to start a normal prompt.\n"
	if got := buf.String(); got != want {
		t.Fatalf("Run() output = %q, want %q", got, want)
	}
}

// TestRunnerRunResumeWithoutArgsListsRecentSessions verifies bare /resume prints recent project-scoped sessions instead of only a usage error.
func TestRunnerRunResumeWithoutArgsListsRecentSessions(t *testing.T) {
	var buf bytes.Buffer
	eng := &recordingEngine{}
	repo := &recordingSessionRepository{
		listRecent: []coresession.Summary{
			{ID: "session-3", ProjectPath: "/repo", UpdatedAt: time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)},
			{ID: "session-2", ProjectPath: "/repo", UpdatedAt: time.Date(2026, 4, 5, 11, 0, 0, 0, time.UTC)},
		},
	}
	runner := NewRunner(eng, console.NewStreamRenderer(console.NewPrinter(&buf)))
	runner.ProjectPath = "/repo"
	runner.SessionManager = runtimesession.NewManager(repo)
	registerSlashCommands(t, runner, NewResumeCommandAdapter(runner))

	if err := runner.Run(context.Background(), []string{"/resume"}); err != nil {
		t.Fatalf("Run(/resume) error = %v", err)
	}

	want := "Recent conversations:\n- session-3\n- session-2\nUse /resume <session-id> <prompt> to continue one.\n"
	if got := buf.String(); got != want {
		t.Fatalf("Run(/resume) output = %q, want %q", got, want)
	}
}

// TestRunnerRunContinueAliasDispatchesResume verifies /continue resolves through the shared registry alias table.
func TestRunnerRunContinueAliasDispatchesResume(t *testing.T) {
	stream := make(chan event.Event, 2)
	stream <- event.Event{
		Type:      event.TypeMessageDelta,
		Timestamp: time.Now(),
		Payload:   event.MessageDeltaPayload{Text: "continued"},
	}
	stream <- event.Event{
		Type:      event.TypeConversationDone,
		Timestamp: time.Now(),
		Payload: event.ConversationDonePayload{
			History: conversation.History{
				Messages: []message.Message{
					{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("before")}},
					{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("new prompt")}},
				},
			},
		},
	}
	close(stream)

	var buf bytes.Buffer
	eng := &recordingEngine{stream: stream}
	repo := &recordingSessionRepository{
		loadResult: coresession.Session{
			ID:          "session-2",
			ProjectPath: "/repo",
			Messages: []message.Message{
				{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("before")}},
			},
		},
	}
	runner := NewRunner(eng, console.NewStreamRenderer(console.NewPrinter(&buf)))
	runner.ProjectPath = "/repo"
	runner.SessionManager = runtimesession.NewManager(repo)
	registerSlashCommands(t, runner, NewResumeCommandAdapter(runner))

	if err := runner.Run(context.Background(), []string{"/continue", "session-2", "new", "prompt"}); err != nil {
		t.Fatalf("Run(/continue) error = %v", err)
	}
	if eng.lastRequest.SessionID != "session-2" {
		t.Fatalf("Run(/continue) session id = %q, want session-2", eng.lastRequest.SessionID)
	}
}

// TestRunnerRunClearStartsFreshSession verifies /clear switches the runner to a fresh session so the next prompt does not reuse old history.
func TestRunnerRunClearStartsFreshSession(t *testing.T) {
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

	repo := &recordingSessionRepository{
		loadResult: coresession.Session{
			ID: "session-old",
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
	runner.SessionID = "session-old"
	runner.ProjectPath = "/repo"
	runner.SessionManager = manager
	runner.AutoSave = autosave
	registerSlashCommands(t, runner, servicecommands.ClearCommand{})

	if err := runner.Run(context.Background(), []string{"/clear"}); err != nil {
		t.Fatalf("Run(/clear) error = %v", err)
	}

	oldSessionID := "session-old"
	if runner.SessionID == "" || runner.SessionID == oldSessionID {
		t.Fatalf("Run(/clear) session id = %q, want fresh session id", runner.SessionID)
	}
	if got := buf.String(); got != "Started a new session.\n" {
		t.Fatalf("Run(/clear) output = %q, want clear confirmation", got)
	}

	buf.Reset()
	if err := runner.Run(context.Background(), []string{"fresh", "prompt"}); err != nil {
		t.Fatalf("Run(prompt after clear) error = %v", err)
	}

	if eng.lastRequest.SessionID != runner.SessionID {
		t.Fatalf("Run(prompt after clear) session id = %q, want %q", eng.lastRequest.SessionID, runner.SessionID)
	}
	if len(eng.lastRequest.Messages) != 1 || eng.lastRequest.Messages[0].Content[0].Text != "fresh prompt" {
		t.Fatalf("Run(prompt after clear) messages = %#v, want fresh prompt only", eng.lastRequest.Messages)
	}
	if len(repo.saved) != 1 {
		t.Fatalf("autosave saved count = %d, want 1", len(repo.saved))
	}
	if repo.saved[0].ID != runner.SessionID {
		t.Fatalf("autosave saved session id = %q, want %q", repo.saved[0].ID, runner.SessionID)
	}
}

// TestRunnerRunClearUpdatesLatestSessionForContinue verifies /clear followed by autosave makes explicit --continue recover the fresh session instead of the old one.
func TestRunnerRunClearUpdatesLatestSessionForContinue(t *testing.T) {
	clearStream := make(chan event.Event, 2)
	clearStream <- event.Event{
		Type:      event.TypeMessageDelta,
		Timestamp: time.Now(),
		Payload: event.MessageDeltaPayload{
			Text: "fresh reply",
		},
	}
	clearStream <- event.Event{
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
	close(clearStream)

	repo := &recordingSessionRepository{
		latestResult: coresession.Session{
			ID:          "session-old",
			ProjectPath: "/repo",
			Messages: []message.Message{
				{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("old prompt")}},
				{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("old reply")}},
			},
		},
	}
	manager := runtimesession.NewManager(repo)
	autosave := runtimesession.NewAutoSave(manager)

	var clearBuf bytes.Buffer
	clearEngine := &recordingEngine{stream: clearStream}
	clearRunner := NewRunner(clearEngine, console.NewStreamRenderer(console.NewPrinter(&clearBuf)))
	clearRunner.ProjectPath = "/repo"
	clearRunner.SessionID = "session-old"
	clearRunner.SessionManager = manager
	clearRunner.AutoSave = autosave
	registerSlashCommands(t, clearRunner, servicecommands.ClearCommand{})

	if err := clearRunner.Run(context.Background(), []string{"/clear"}); err != nil {
		t.Fatalf("Run(/clear) error = %v", err)
	}
	freshSessionID := clearRunner.SessionID
	if freshSessionID == "" || freshSessionID == "session-old" {
		t.Fatalf("Run(/clear) session id = %q, want fresh session id", freshSessionID)
	}

	clearBuf.Reset()
	if err := clearRunner.Run(context.Background(), []string{"fresh", "prompt"}); err != nil {
		t.Fatalf("Run(prompt after clear) error = %v", err)
	}

	continueStream := make(chan event.Event, 2)
	continueStream <- event.Event{
		Type:      event.TypeMessageDelta,
		Timestamp: time.Now(),
		Payload: event.MessageDeltaPayload{
			Text: "continued fresh reply",
		},
	}
	continueStream <- event.Event{
		Type:      event.TypeConversationDone,
		Timestamp: time.Now(),
		Payload: event.ConversationDonePayload{
			History: conversation.History{
				Messages: []message.Message{
					{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("fresh prompt")}},
					{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("fresh reply")}},
					{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("follow-up")}},
					{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("continued fresh reply")}},
				},
			},
		},
	}
	close(continueStream)

	var continueBuf bytes.Buffer
	continueEngine := &recordingEngine{stream: continueStream}
	continueRunner := NewRunner(continueEngine, console.NewStreamRenderer(console.NewPrinter(&continueBuf)))
	continueRunner.ProjectPath = "/repo"
	continueRunner.SessionManager = manager
	continueRunner.AutoSave = autosave

	if err := continueRunner.Run(context.Background(), []string{"--continue", "follow-up"}); err != nil {
		t.Fatalf("Run(--continue) error = %v", err)
	}

	if continueEngine.lastRequest.SessionID != freshSessionID {
		t.Fatalf("Run(--continue) session id = %q, want latest fresh session %q", continueEngine.lastRequest.SessionID, freshSessionID)
	}
	if len(continueEngine.lastRequest.Messages) != 3 {
		t.Fatalf("Run(--continue) request message count = %d, want 3", len(continueEngine.lastRequest.Messages))
	}
	if continueEngine.lastRequest.Messages[0].Content[0].Text != "fresh prompt" {
		t.Fatalf("Run(--continue) first message = %#v, want fresh prompt history", continueEngine.lastRequest.Messages[0])
	}
	if continueEngine.lastRequest.Messages[2].Content[0].Text != "follow-up" {
		t.Fatalf("Run(--continue) last message = %#v, want follow-up", continueEngine.lastRequest.Messages[2])
	}
}

// TestRunnerRunClearPreservesResumeOfOlderSession verifies /clear does not break explicit /resume recovery of an older saved session.
func TestRunnerRunClearPreservesResumeOfOlderSession(t *testing.T) {
	clearStream := make(chan event.Event, 2)
	clearStream <- event.Event{
		Type:      event.TypeMessageDelta,
		Timestamp: time.Now(),
		Payload: event.MessageDeltaPayload{
			Text: "fresh reply",
		},
	}
	clearStream <- event.Event{
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
	close(clearStream)

	repo := &recordingSessionRepository{
		loadResult: coresession.Session{
			ID:          "session-old",
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
	eng := &recordingEngine{stream: clearStream}
	runner := NewRunner(eng, console.NewStreamRenderer(console.NewPrinter(&buf)))
	runner.ProjectPath = "/repo"
	runner.SessionID = "session-old"
	runner.SessionManager = manager
	runner.AutoSave = autosave
	registerSlashCommands(t, runner, servicecommands.ClearCommand{}, NewResumeCommandAdapter(runner))

	if err := runner.Run(context.Background(), []string{"/clear"}); err != nil {
		t.Fatalf("Run(/clear) error = %v", err)
	}
	if err := runner.Run(context.Background(), []string{"fresh", "prompt"}); err != nil {
		t.Fatalf("Run(prompt after clear) error = %v", err)
	}

	resumeStream := make(chan event.Event, 2)
	resumeStream <- event.Event{
		Type:      event.TypeMessageDelta,
		Timestamp: time.Now(),
		Payload: event.MessageDeltaPayload{
			Text: "resumed reply",
		},
	}
	resumeStream <- event.Event{
		Type:      event.TypeConversationDone,
		Timestamp: time.Now(),
		Payload: event.ConversationDonePayload{
			History: conversation.History{
				Messages: []message.Message{
					{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("old prompt")}},
					{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("old reply")}},
					{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("resume prompt")}},
					{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("resumed reply")}},
				},
			},
		},
	}
	close(resumeStream)
	eng.stream = resumeStream

	buf.Reset()
	if err := runner.Run(context.Background(), []string{"/resume", "session-old", "resume", "prompt"}); err != nil {
		t.Fatalf("Run(/resume after clear) error = %v", err)
	}

	if eng.lastRequest.SessionID != "session-old" {
		t.Fatalf("Run(/resume after clear) session id = %q, want session-old", eng.lastRequest.SessionID)
	}
	if len(eng.lastRequest.Messages) != 3 {
		t.Fatalf("Run(/resume after clear) request message count = %d, want 3", len(eng.lastRequest.Messages))
	}
	if eng.lastRequest.Messages[0].Content[0].Text != "old prompt" {
		t.Fatalf("Run(/resume after clear) first message = %#v, want old prompt history", eng.lastRequest.Messages[0])
	}
	if eng.lastRequest.Messages[2].Content[0].Text != "resume prompt" {
		t.Fatalf("Run(/resume after clear) last message = %#v, want resume prompt", eng.lastRequest.Messages[2])
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
	registerSlashCommands(t, runner, NewResumeCommandAdapter(runner))

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
	registerSlashCommands(t, runner, NewResumeCommandAdapter(runner))

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
	registerSlashCommands(t, runner, NewResumeCommandAdapter(runner))

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

// TestRunnerRunContinueRequiresExistingSession verifies explicit --continue reports a stable error instead of creating a new session.
func TestRunnerRunContinueRequiresExistingSession(t *testing.T) {
	var buf bytes.Buffer
	eng := &recordingEngine{}
	manager := runtimesession.NewManager(&recordingSessionRepository{latestErr: coresession.ErrSessionNotFound})
	runner := NewRunner(eng, console.NewStreamRenderer(console.NewPrinter(&buf)))
	runner.ProjectPath = "/repo"
	runner.SessionManager = manager

	if err := runner.Run(context.Background(), []string{"--continue", "follow-up"}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if eng.lastRequest.SessionID != "" || len(eng.lastRequest.Messages) != 0 {
		t.Fatalf("engine should not be called for missing continue session, got request %#v", eng.lastRequest)
	}
	if got := buf.String(); got != continueNotFoundMessage+"\n" {
		t.Fatalf("Run() output = %q, want continue missing-session error", got)
	}
}

// TestRunnerRunContinueForksLatestSession verifies explicit continue can fork the latest session into a new target id.
func TestRunnerRunContinueForksLatestSession(t *testing.T) {
	stream := make(chan event.Event, 2)
	stream <- event.Event{
		Type:      event.TypeMessageDelta,
		Timestamp: time.Now(),
		Payload: event.MessageDeltaPayload{
			Text: "forked reply",
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
					{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("forked reply")}},
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

	if err := runner.Run(context.Background(), []string{"--fork-session", "--continue", "follow-up"}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if eng.lastRequest.SessionID == "" {
		t.Fatal("Run() request.SessionID = empty, want forked session id")
	}
	if eng.lastRequest.SessionID == "session-latest" {
		t.Fatalf("Run() request.SessionID = %q, want new forked session id", eng.lastRequest.SessionID)
	}
	if len(repo.saved) != 2 {
		t.Fatalf("saved count = %d, want 2 (fork + autosave)", len(repo.saved))
	}
	if repo.saved[0].ID != eng.lastRequest.SessionID {
		t.Fatalf("fork saved session id = %q, want %q", repo.saved[0].ID, eng.lastRequest.SessionID)
	}
	if repo.saved[1].ID != eng.lastRequest.SessionID {
		t.Fatalf("autosave saved session id = %q, want %q", repo.saved[1].ID, eng.lastRequest.SessionID)
	}
	if repo.saved[1].ProjectPath != "/repo" {
		t.Fatalf("autosave saved project path = %q, want /repo", repo.saved[1].ProjectPath)
	}
}

// TestRunnerRunResumeForksSession verifies /resume can clone history into a new session before continuing.
func TestRunnerRunResumeForksSession(t *testing.T) {
	stream := make(chan event.Event, 2)
	stream <- event.Event{
		Type:      event.TypeMessageDelta,
		Timestamp: time.Now(),
		Payload: event.MessageDeltaPayload{
			Text: "forked resume reply",
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
					{Role: message.RoleAssistant, Content: []message.ContentPart{message.TextPart("forked resume reply")}},
				},
			},
		},
	}
	close(stream)

	repo := &recordingSessionRepository{
		loadResult: coresession.Session{
			ID:          "session-2",
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
	runner.SessionManager = manager
	runner.AutoSave = autosave
	registerSlashCommands(t, runner, NewResumeCommandAdapter(runner))

	if err := runner.Run(context.Background(), []string{"--fork-session", "/resume", "session-2", "new", "prompt"}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if eng.lastRequest.SessionID == "" {
		t.Fatal("Run() request.SessionID = empty, want forked session id")
	}
	if eng.lastRequest.SessionID == "session-2" {
		t.Fatalf("Run() request.SessionID = %q, want new forked session id", eng.lastRequest.SessionID)
	}
	if len(repo.saved) != 2 {
		t.Fatalf("saved count = %d, want 2 (fork + autosave)", len(repo.saved))
	}
	if repo.saved[0].ID != eng.lastRequest.SessionID {
		t.Fatalf("fork saved session id = %q, want %q", repo.saved[0].ID, eng.lastRequest.SessionID)
	}
	if repo.saved[1].ID != eng.lastRequest.SessionID {
		t.Fatalf("autosave saved session id = %q, want %q", repo.saved[1].ID, eng.lastRequest.SessionID)
	}
}
