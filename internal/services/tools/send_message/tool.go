package send_message

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	"github.com/sheepzhao/claude-code-go/internal/platform/mailbox"
	"github.com/sheepzhao/claude-code-go/internal/platform/team"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const Name = "SendMessage"

// Tool implements the SendMessage tool for sending messages between agents
// within a team. It supports direct messages to a teammate by name and
// broadcast to all team members.
type Tool struct {
	homeDir string
}

// NewTool creates a SendMessage tool that operates under the given home directory.
func NewTool(homeDir string) *Tool {
	return &Tool{homeDir: homeDir}
}

func (t *Tool) Name() string { return Name }

func (t *Tool) Description() string {
	return "Send a message to another agent within the same team. " +
		"Use this to communicate with teammates spawned by the Agent tool. " +
		"Set to \"*\" to broadcast to all teammates."
}

// Input defines the SendMessage tool input schema.
type Input struct {
	TeamName string `json:"team_name"`
	To       string `json:"to"`
	Message  string `json:"message"`
	Summary  string `json:"summary,omitempty"`
}

// RoutingInfo describes the sender and target metadata for a sent message.
type RoutingInfo struct {
	Sender  string `json:"sender"`
	Target  string `json:"target"`
	Summary string `json:"summary,omitempty"`
	Content string `json:"content,omitempty"`
}

// Output defines the SendMessage tool output.
type Output struct {
	Success    bool         `json:"success"`
	Message    string       `json:"message"`
	Routing    *RoutingInfo `json:"routing,omitempty"`
	Recipients []string     `json:"recipients,omitempty"`
}

func (t *Tool) InputSchema() coretool.InputSchema {
	return coretool.InputSchema{
		Properties: map[string]coretool.FieldSchema{
			"team_name": {
				Type:        coretool.ValueKindString,
				Description: "The team name this message belongs to.",
				Required:    true,
			},
			"to": {
				Type:        coretool.ValueKindString,
				Description: "Recipient: teammate name, or \"*\" for broadcast to all teammates.",
				Required:    true,
			},
			"message": {
				Type:        coretool.ValueKindString,
				Description: "Plain text message content.",
				Required:    true,
			},
			"summary": {
				Type:        coretool.ValueKindString,
				Description: "A 5-10 word summary shown as a preview in the UI (required when message is a string).",
			},
		},
	}
}

func (t *Tool) IsReadOnly() bool       { return true }
func (t *Tool) IsConcurrencySafe() bool { return true }

// senderName returns the current agent's display name, defaulting to "team-lead".
func senderName() string {
	if name := os.Getenv("CLAUDE_CODE_AGENT_NAME"); name != "" {
		return name
	}
	return "team-lead"
}

// Invoke routes the message to the specified recipient or broadcasts to all teammates.
func (t *Tool) Invoke(ctx context.Context, call coretool.Call) (coretool.Result, error) {
	input, err := coretool.DecodeInput[Input](t.InputSchema(), call.Input)
	if err != nil {
		return coretool.Result{Error: fmt.Sprintf("invalid input: %v", err)}, nil
	}

	// Validate and normalize input
	teamName := strings.TrimSpace(input.TeamName)
	if teamName == "" {
		return coretool.Result{Error: "team_name is required for SendMessage"}, nil
	}

	to := strings.TrimSpace(input.To)
	if to == "" {
		return coretool.Result{Error: "to must not be empty"}, nil
	}
	if strings.Contains(to, "@") {
		return coretool.Result{Error: "to must be a bare teammate name or \"*\" — there is only one team per session"}, nil
	}

	msg := strings.TrimSpace(input.Message)
	if msg == "" {
		return coretool.Result{Error: "message must not be empty"}, nil
	}

	summary := strings.TrimSpace(input.Summary)
	if summary == "" {
		return coretool.Result{Error: "summary is required when message is a string"}, nil
	}

	sender := senderName()

	// Broadcast path
	if to == "*" {
		return t.handleBroadcast(teamName, msg, summary, sender)
	}

	// Direct message path
	return t.handleMessage(to, teamName, msg, summary, sender)
}

// handleMessage sends a direct message to a single recipient.
func (t *Tool) handleMessage(recipientName, teamName, content, summary, sender string) (coretool.Result, error) {
	mailboxMsg := mailbox.Message{
		From:      sender,
		Text:      content,
		Summary:   summary,
		Timestamp: time.Now().Format(time.RFC3339),
	}

	if err := mailbox.WriteToMailbox(recipientName, mailboxMsg, teamName, t.homeDir); err != nil {
		logger.ErrorCF("send_message", "failed to write to mailbox", map[string]any{
			"recipient": recipientName,
			"team":      teamName,
			"error":     err.Error(),
		})
		return coretool.Result{Error: fmt.Sprintf("failed to send message: %v", err)}, nil
	}

	logger.DebugCF("send_message", "message sent", map[string]any{
		"from": sender,
		"to":   recipientName,
		"team": teamName,
	})

	data, _ := json.Marshal(Output{
		Success: true,
		Message: fmt.Sprintf("Message sent to %s's inbox", recipientName),
		Routing: &RoutingInfo{
			Sender:  sender,
			Target:  fmt.Sprintf("@%s", recipientName),
			Summary: summary,
			Content: content,
		},
	})
	return coretool.Result{Output: string(data)}, nil
}

// handleBroadcast sends a message to all team members except the sender.
func (t *Tool) handleBroadcast(teamName, content, summary, sender string) (coretool.Result, error) {
	teamFile, err := team.ReadTeamFile(t.homeDir, teamName)
	if err != nil {
		logger.ErrorCF("send_message", "failed to read team file for broadcast", map[string]any{
			"team":  teamName,
			"error": err.Error(),
		})
		return coretool.Result{Error: fmt.Sprintf("failed to read team %q: %v", teamName, err)}, nil
	}
	if teamFile == nil {
		return coretool.Result{Error: fmt.Sprintf("team %q does not exist", teamName)}, nil
	}

	senderLower := strings.ToLower(sender)
	var recipients []string
	for _, member := range teamFile.Members {
		if strings.ToLower(member.Name) == senderLower {
			continue
		}
		recipients = append(recipients, member.Name)
	}

	if len(recipients) == 0 {
		data, _ := json.Marshal(Output{
			Success:    true,
			Message:    "No teammates to broadcast to (you are the only team member)",
			Recipients: []string{},
		})
		return coretool.Result{Output: string(data)}, nil
	}

	mailboxMsg := mailbox.Message{
		From:      sender,
		Text:      content,
		Summary:   summary,
		Timestamp: time.Now().Format(time.RFC3339),
	}

	var errors []string
	for _, recipientName := range recipients {
		if err := mailbox.WriteToMailbox(recipientName, mailboxMsg, teamName, t.homeDir); err != nil {
			logger.ErrorCF("send_message", "failed to broadcast to recipient", map[string]any{
				"recipient": recipientName,
				"team":      teamName,
				"error":     err.Error(),
			})
			errors = append(errors, recipientName)
		}
	}

	if len(errors) > 0 {
		return coretool.Result{Error: fmt.Sprintf("broadcast partially failed: could not deliver to %s", strings.Join(errors, ", "))}, nil
	}

	logger.DebugCF("send_message", "broadcast sent", map[string]any{
		"from":         sender,
		"team":         teamName,
		"recipient_count": len(recipients),
	})

	data, _ := json.Marshal(Output{
		Success:    true,
		Message:    fmt.Sprintf("Message broadcast to %d teammate(s): %s", len(recipients), strings.Join(recipients, ", ")),
		Recipients: recipients,
		Routing: &RoutingInfo{
			Sender:  sender,
			Target:  "@team",
			Summary: summary,
			Content: content,
		},
	})
	return coretool.Result{Output: string(data)}, nil
}
