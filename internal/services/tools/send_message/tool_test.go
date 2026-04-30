package send_message

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	"github.com/sheepzhao/claude-code-go/internal/platform/mailbox"
	"github.com/sheepzhao/claude-code-go/internal/platform/team"
)

func testHomeDir(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

func TestName(t *testing.T) {
	tool := NewTool("")
	if tool.Name() != Name {
		t.Fatalf("expected name %q, got %q", Name, tool.Name())
	}
}

func TestDescription(t *testing.T) {
	tool := NewTool("")
	if tool.Description() == "" {
		t.Fatal("expected non-empty description")
	}
}

func TestInputSchema(t *testing.T) {
	tool := NewTool("")
	schema := tool.InputSchema()
	if _, ok := schema.Properties["team_name"]; !ok {
		t.Fatal("expected team_name property")
	}
	if !schema.Properties["team_name"].Required {
		t.Fatal("expected team_name to be required")
	}
	if _, ok := schema.Properties["to"]; !ok {
		t.Fatal("expected to property")
	}
	if !schema.Properties["to"].Required {
		t.Fatal("expected to to be required")
	}
	if _, ok := schema.Properties["message"]; !ok {
		t.Fatal("expected message property")
	}
	if !schema.Properties["message"].Required {
		t.Fatal("expected message to be required")
	}
	if _, ok := schema.Properties["summary"]; !ok {
		t.Fatal("expected summary property")
	}
	if schema.Properties["summary"].Required {
		t.Fatal("expected summary to be optional in schema")
	}
}

func TestIsReadOnly(t *testing.T) {
	tool := NewTool("")
	if !tool.IsReadOnly() {
		t.Fatal("expected IsReadOnly to return true")
	}
}

func TestIsConcurrencySafe(t *testing.T) {
	tool := NewTool("")
	if !tool.IsConcurrencySafe() {
		t.Fatal("expected IsConcurrencySafe to return true")
	}
}

func TestInvoke_DirectMessage(t *testing.T) {
	homeDir := testHomeDir(t)
	tool := NewTool(homeDir)

	// Create a team so the config.json exists and has an inbox directory structure.
	teamName := "test-team"
	_ = team.WriteTeamFile(homeDir, teamName, &team.TeamFile{
		Name:      teamName,
		LeadAgentID: "team-lead@test-team",
		Members: []team.TeamMember{
			{AgentID: "team-lead@test-team", Name: "team-lead", AgentType: "team-lead"},
			{AgentID: "helper@test-team", Name: "helper", AgentType: "general-purpose"},
		},
	})

	call := coretool.Call{
		Input: map[string]any{
			"team_name": teamName,
			"to":        "helper",
			"message":   "Hello from team lead",
			"summary":   "Greeting message",
		},
	}
	result, err := tool.Invoke(context.Background(), call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected result error: %s", result.Error)
	}

	var output Output
	if err := json.Unmarshal([]byte(result.Output), &output); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}
	if !output.Success {
		t.Fatal("expected success=true")
	}
	if output.Routing == nil {
		t.Fatal("expected routing info")
	}
	if output.Routing.Target != "@helper" {
		t.Fatalf("expected target @helper, got %s", output.Routing.Target)
	}
	if output.Routing.Sender != "team-lead" {
		t.Fatalf("expected sender team-lead, got %s", output.Routing.Sender)
	}

	// Verify the message was written to the recipient's mailbox.
	msgs, err := mailbox.ReadMailbox("helper", teamName, homeDir)
	if err != nil {
		t.Fatalf("failed to read mailbox: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message in mailbox, got %d", len(msgs))
	}
	if msgs[0].Text != "Hello from team lead" {
		t.Fatalf("expected message text, got %s", msgs[0].Text)
	}
	if msgs[0].Summary != "Greeting message" {
		t.Fatalf("expected summary, got %s", msgs[0].Summary)
	}
}

func TestInvoke_Broadcast(t *testing.T) {
	homeDir := testHomeDir(t)
	tool := NewTool(homeDir)

	teamName := "broadcast-team"
	_ = team.WriteTeamFile(homeDir, teamName, &team.TeamFile{
		Name:      teamName,
		LeadAgentID: "team-lead@broadcast-team",
		Members: []team.TeamMember{
			{AgentID: "team-lead@broadcast-team", Name: "team-lead", AgentType: "team-lead"},
			{AgentID: "alice@broadcast-team", Name: "alice", AgentType: "general-purpose"},
			{AgentID: "bob@broadcast-team", Name: "bob", AgentType: "general-purpose"},
		},
	})

	call := coretool.Call{
		Input: map[string]any{
			"team_name": teamName,
			"to":        "*",
			"message":   "Broadcast announcement",
			"summary":   "Team update",
		},
	}
	result, err := tool.Invoke(context.Background(), call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected result error: %s", result.Error)
	}

	var output Output
	if err := json.Unmarshal([]byte(result.Output), &output); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}
	if !output.Success {
		t.Fatal("expected success=true")
	}
	if len(output.Recipients) != 2 {
		t.Fatalf("expected 2 recipients (skipping self), got %d: %v", len(output.Recipients), output.Recipients)
	}
	if output.Routing.Target != "@team" {
		t.Fatalf("expected target @team, got %s", output.Routing.Target)
	}

	// Verify both alice and bob received the message.
	for _, name := range []string{"alice", "bob"} {
		msgs, err := mailbox.ReadMailbox(name, teamName, homeDir)
		if err != nil {
			t.Fatalf("failed to read %s mailbox: %v", name, err)
		}
		if len(msgs) != 1 {
			t.Fatalf("expected 1 message in %s mailbox, got %d", name, len(msgs))
		}
		if msgs[0].Text != "Broadcast announcement" {
			t.Fatalf("expected broadcast text in %s mailbox", name)
		}
	}

	// Verify team-lead (sender) did not receive the message.
	msgs, err := mailbox.ReadMailbox("team-lead", teamName, homeDir)
	if err != nil {
		t.Fatalf("failed to read team-lead mailbox: %v", err)
	}
	if len(msgs) != 0 {
		t.Fatal("expected sender to not receive broadcast")
	}
}

func TestInvoke_EmptyTeamName(t *testing.T) {
	homeDir := testHomeDir(t)
	tool := NewTool(homeDir)

	call := coretool.Call{
		Input: map[string]any{
			"team_name": "   ",
			"to":        "helper",
			"message":   "test",
			"summary":   "test",
		},
	}
	result, err := tool.Invoke(context.Background(), call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Fatal("expected error for empty team_name")
	}
}

func TestInvoke_EmptyTo(t *testing.T) {
	homeDir := testHomeDir(t)
	tool := NewTool(homeDir)

	call := coretool.Call{
		Input: map[string]any{
			"team_name": "team",
			"to":        "   ",
			"message":   "test",
			"summary":   "test",
		},
	}
	result, err := tool.Invoke(context.Background(), call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == "" || !strings.Contains(result.Error, "must not be empty") {
		t.Fatalf("expected 'must not be empty' error, got: %s", result.Error)
	}
}

func TestInvoke_ToWithAtSign(t *testing.T) {
	homeDir := testHomeDir(t)
	tool := NewTool(homeDir)

	call := coretool.Call{
		Input: map[string]any{
			"team_name": "team",
			"to":        "helper@team",
			"message":   "test",
			"summary":   "test",
		},
	}
	result, err := tool.Invoke(context.Background(), call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == "" || !strings.Contains(result.Error, "bare teammate name") {
		t.Fatalf("expected 'bare teammate name' error, got: %s", result.Error)
	}
}

func TestInvoke_EmptyMessage(t *testing.T) {
	homeDir := testHomeDir(t)
	tool := NewTool(homeDir)

	call := coretool.Call{
		Input: map[string]any{
			"team_name": "team",
			"to":        "helper",
			"message":   "   ",
			"summary":   "test",
		},
	}
	result, err := tool.Invoke(context.Background(), call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == "" || !strings.Contains(result.Error, "must not be empty") {
		t.Fatalf("expected 'must not be empty' error, got: %s", result.Error)
	}
}

func TestInvoke_MissingSummary(t *testing.T) {
	homeDir := testHomeDir(t)
	tool := NewTool(homeDir)

	call := coretool.Call{
		Input: map[string]any{
			"team_name": "team",
			"to":        "helper",
			"message":   "test message",
			"summary":   "   ",
		},
	}
	result, err := tool.Invoke(context.Background(), call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == "" || !strings.Contains(result.Error, "summary is required") {
		t.Fatalf("expected 'summary is required' error, got: %s", result.Error)
	}
}

func TestInvoke_TeamNotExist(t *testing.T) {
	homeDir := testHomeDir(t)
	tool := NewTool(homeDir)

	call := coretool.Call{
		Input: map[string]any{
			"team_name": "nonexistent",
			"to":        "*",
			"message":   "test broadcast",
			"summary":   "test",
		},
	}
	result, err := tool.Invoke(context.Background(), call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == "" || !strings.Contains(result.Error, "does not exist") {
		t.Fatalf("expected 'does not exist' error, got: %s", result.Error)
	}
}

func TestInvoke_BroadcastOnlySelf(t *testing.T) {
	homeDir := testHomeDir(t)
	tool := NewTool(homeDir)

	teamName := "solo-team"
	_ = team.WriteTeamFile(homeDir, teamName, &team.TeamFile{
		Name:      teamName,
		LeadAgentID: "team-lead@solo-team",
		Members: []team.TeamMember{
			{AgentID: "team-lead@solo-team", Name: "team-lead", AgentType: "team-lead"},
		},
	})

	call := coretool.Call{
		Input: map[string]any{
			"team_name": teamName,
			"to":        "*",
			"message":   "broadcast to self only",
			"summary":   "test",
		},
	}
	result, err := tool.Invoke(context.Background(), call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected result error: %s", result.Error)
	}

	var output Output
	if err := json.Unmarshal([]byte(result.Output), &output); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}
	if !output.Success {
		t.Fatal("expected success=true even with no recipients")
	}
	if len(output.Recipients) != 0 {
		t.Fatalf("expected 0 recipients, got %d", len(output.Recipients))
	}
}

func TestInvoke_SenderFromEnv(t *testing.T) {
	homeDir := testHomeDir(t)
	tool := NewTool(homeDir)

	// Set the agent name env var
	os.Setenv("CLAUDE_CODE_AGENT_NAME", "custom-sender")
	defer os.Unsetenv("CLAUDE_CODE_AGENT_NAME")

	teamName := "sender-test-team"
	_ = team.WriteTeamFile(homeDir, teamName, &team.TeamFile{
		Name:      teamName,
		LeadAgentID: "custom-sender@sender-test-team",
		Members: []team.TeamMember{
			{AgentID: "custom-sender@sender-test-team", Name: "custom-sender", AgentType: "team-lead"},
			{AgentID: "helper@sender-test-team", Name: "helper", AgentType: "general-purpose"},
		},
	})

	call := coretool.Call{
		Input: map[string]any{
			"team_name": teamName,
			"to":        "helper",
			"message":   "Message from custom sender",
			"summary":   "Custom sender test",
		},
	}
	result, err := tool.Invoke(context.Background(), call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected result error: %s", result.Error)
	}

	var output Output
	if err := json.Unmarshal([]byte(result.Output), &output); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}
	if output.Routing.Sender != "custom-sender" {
		t.Fatalf("expected sender custom-sender, got %s", output.Routing.Sender)
	}
}

func TestInvoke_MissingTeamNameField(t *testing.T) {
	homeDir := testHomeDir(t)
	tool := NewTool(homeDir)

	call := coretool.Call{
		Input: map[string]any{
			"to":      "helper",
			"message": "test",
			"summary": "test",
		},
	}
	result, err := tool.Invoke(context.Background(), call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Fatal("expected schema decode error for missing team_name")
	}
}

func TestInvoke_MissingToField(t *testing.T) {
	homeDir := testHomeDir(t)
	tool := NewTool(homeDir)

	call := coretool.Call{
		Input: map[string]any{
			"team_name": "team",
			"message":   "test",
			"summary":   "test",
		},
	}
	result, err := tool.Invoke(context.Background(), call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Fatal("expected schema decode error for missing to")
	}
}

func TestInvoke_DirectMessageToTeamLead(t *testing.T) {
	homeDir := testHomeDir(t)
	tool := NewTool(homeDir)

	teamName := "msg-team"
	_ = team.WriteTeamFile(homeDir, teamName, &team.TeamFile{
		Name:      teamName,
		LeadAgentID: "team-lead@msg-team",
		Members: []team.TeamMember{
			{AgentID: "team-lead@msg-team", Name: "team-lead", AgentType: "team-lead"},
			{AgentID: "helper@msg-team", Name: "helper", AgentType: "general-purpose"},
		},
	})

	// helper sends a message to team-lead
	os.Setenv("CLAUDE_CODE_AGENT_NAME", "helper")
	defer os.Unsetenv("CLAUDE_CODE_AGENT_NAME")

	call := coretool.Call{
		Input: map[string]any{
			"team_name": teamName,
			"to":        "team-lead",
			"message":   "Reporting back to lead",
			"summary":   "Status update",
		},
	}
	result, err := tool.Invoke(context.Background(), call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected result error: %s", result.Error)
	}

	// Verify message in team-lead's mailbox.
	msgs, err := mailbox.ReadMailbox("team-lead", teamName, homeDir)
	if err != nil {
		t.Fatalf("failed to read mailbox: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message in team-lead mailbox, got %d", len(msgs))
	}
	if msgs[0].From != "helper" {
		t.Fatalf("expected from=helper, got %s", msgs[0].From)
	}
}

func TestInvoke_MessageTimestamp(t *testing.T) {
	homeDir := testHomeDir(t)
	tool := NewTool(homeDir)

	teamName := "timestamp-team"
	_ = team.WriteTeamFile(homeDir, teamName, &team.TeamFile{
		Name:      teamName,
		LeadAgentID: "team-lead@timestamp-team",
		Members: []team.TeamMember{
			{AgentID: "team-lead@timestamp-team", Name: "team-lead", AgentType: "team-lead"},
			{AgentID: "helper@timestamp-team", Name: "helper", AgentType: "general-purpose"},
		},
	})

	call := coretool.Call{
		Input: map[string]any{
			"team_name": teamName,
			"to":        "helper",
			"message":   "Check timestamp",
			"summary":   "Timestamp verification",
		},
	}
	result, err := tool.Invoke(context.Background(), call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected result error: %s", result.Error)
	}

	// Verify the message has a non-empty timestamp.
	msgs, err := mailbox.ReadMailbox("helper", teamName, homeDir)
	if err != nil {
		t.Fatalf("failed to read mailbox: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Timestamp == "" {
		t.Fatal("expected non-empty timestamp")
	}
	// Verify Read is false (unread message).
	if msgs[0].Read {
		t.Fatal("expected new message to be unread")
	}
}

func TestInvoke_SenderNameNoEnv(t *testing.T) {
	homeDir := testHomeDir(t)
	tool := NewTool(homeDir)

	// Ensure no env var is set.
	os.Unsetenv("CLAUDE_CODE_AGENT_NAME")

	teamName := "default-sender-team"
	_ = team.WriteTeamFile(homeDir, teamName, &team.TeamFile{
		Name:      teamName,
		LeadAgentID: "team-lead@default-sender-team",
		Members: []team.TeamMember{
			{AgentID: "team-lead@default-sender-team", Name: "team-lead", AgentType: "team-lead"},
			{AgentID: "helper@default-sender-team", Name: "helper", AgentType: "general-purpose"},
		},
	})

	call := coretool.Call{
		Input: map[string]any{
			"team_name": teamName,
			"to":        "helper",
			"message":   "Default sender test",
			"summary":   "Default",
		},
	}
	result, err := tool.Invoke(context.Background(), call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected result error: %s", result.Error)
	}

	var output Output
	if err := json.Unmarshal([]byte(result.Output), &output); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}
	if output.Routing.Sender != "team-lead" {
		t.Fatalf("expected default sender team-lead, got %s", output.Routing.Sender)
	}
}

func TestInvoke_BroadcastSkipsSelfCaseInsensitive(t *testing.T) {
	homeDir := testHomeDir(t)
	tool := NewTool(homeDir)

	teamName := "case-team"
	_ = team.WriteTeamFile(homeDir, teamName, &team.TeamFile{
		Name:      teamName,
		LeadAgentID: "Team-Lead@case-team",
		Members: []team.TeamMember{
			{AgentID: "Team-Lead@case-team", Name: "Team-Lead", AgentType: "team-lead"},
		},
	})

	// Set agent name in different case.
	os.Setenv("CLAUDE_CODE_AGENT_NAME", "team-lead")
	defer os.Unsetenv("CLAUDE_CODE_AGENT_NAME")

	call := coretool.Call{
		Input: map[string]any{
			"team_name": teamName,
			"to":        "*",
			"message":   "Case test",
			"summary":   "Case",
		},
	}
	result, err := tool.Invoke(context.Background(), call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected result error: %s", result.Error)
	}

	var output Output
	if err := json.Unmarshal([]byte(result.Output), &output); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}
	// Self should be skipped even with different case.
	if len(output.Recipients) != 0 {
		t.Fatalf("expected 0 recipients (self skipped case-insensitively), got %d", len(output.Recipients))
	}
}

func TestInvoke_BroadcastMailboxDirCreated(t *testing.T) {
	homeDir := testHomeDir(t)
	tool := NewTool(homeDir)

	teamName := "mailbox-dir-team"
	_ = team.WriteTeamFile(homeDir, teamName, &team.TeamFile{
		Name:      teamName,
		LeadAgentID: "team-lead@mailbox-dir-team",
		Members: []team.TeamMember{
			{AgentID: "team-lead@mailbox-dir-team", Name: "team-lead", AgentType: "team-lead"},
			{AgentID: "alice@mailbox-dir-team", Name: "alice", AgentType: "general-purpose"},
		},
	})

	// Verify mailbox directory does not exist before.
	safeTeam := strings.ToLower(teamName)
	inboxDir := filepath.Join(homeDir, ".claude", "teams", safeTeam, "inboxes")
	if _, err := os.Stat(inboxDir); !os.IsNotExist(err) {
		t.Log("inbox dir already exists before test")
	}

	call := coretool.Call{
		Input: map[string]any{
			"team_name": teamName,
			"to":        "alice",
			"message":   "Test mailbox creation",
			"summary":   "Mailbox dir",
		},
	}
	result, err := tool.Invoke(context.Background(), call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected result error: %s", result.Error)
	}

	// Verify mailbox directory was created.
	info, err := os.Stat(inboxDir)
	if err != nil {
		t.Fatalf("inbox dir was not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("inbox path is not a directory")
	}
}
