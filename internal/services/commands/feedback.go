package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/internal/core/featureflag"
	"github.com/sheepzhao/claude-code-go/internal/services/feedback"
	"github.com/sheepzhao/claude-code-go/internal/services/policylimits"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const feedbackCommandFallback = "Product feedback submission is not available in Claude Code Go yet. Interactive feedback forms, privacy-aware routing, and upstream report delivery remain unmigrated."

// FeedbackCommand exposes the /feedback behaviour. When FlagFeedback is
// enabled and PolicyLimits allows it, the command collects user input and
// submits it to the Anthropic feedback API. Otherwise it returns a fallback.
type FeedbackCommand struct{}

// Metadata returns the canonical slash descriptor for /feedback.
func (c FeedbackCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "feedback",
		Aliases:     []string{"bug"},
		Description: "Submit feedback about Claude Code",
		Usage:       "/feedback [report]",
	}
}

// Execute runs the feedback command. With FlagFeedback enabled, it collects
// the user's description (from args or interactively) and submits to the API.
func (c FeedbackCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	if allowed, reason := policylimits.IsAllowed(policylimits.ActionAllowProductFeedback); !allowed {
		return command.Result{Output: reason}, nil
	}

	if !featureflag.IsEnabled(featureflag.FlagFeedback) {
		return c.fallbackResult(ctx, args)
	}

	description := strings.Join(args.Raw, " ")

	if strings.TrimSpace(description) == "" {
		return command.Result{
			Output: "Please describe the issue you'd like to report. Usage: /feedback <description>",
		}, nil
	}

	// Build feedback data.
	now := feedback.NowISO()
	data := feedback.FeedbackData{
		DateTime:    now,
		Description: description,
		Platform:    "unknown",
		Version:     "unknown",
	}

	logger.DebugCF("commands", "submitting feedback", map[string]any{
		"description_len": len(description),
		"timestamp":       now,
	})

	// Generate a fallback title from the description.
	// Haiku-based title generation is deferred until the dependency
	// cycle between commands and haiku (via anthropic → quota_probe) is resolved.
	titleText := feedback.CreateFallbackTitle(description)

	// Submit to API.
	result := feedback.SubmitFeedback(ctx, data, nil, "")
	if result.Success {
		cfg := feedback.DefaultConfig()
		issueURL := feedback.CreateGitHubIssueURL(cfg, result.FeedbackID, titleText, description, nil)
		var b strings.Builder
		b.WriteString(fmt.Sprintf("Thank you for your report! Feedback ID: %s\n", result.FeedbackID))
		b.WriteString("Press Enter to open your browser and draft a GitHub issue, or any other key to close.\n")
		b.WriteString(fmt.Sprintf("GitHub Issue URL: %s\n", issueURL))
		return command.Result{Output: b.String()}, nil
	}

	if result.IsZDROrg {
		return command.Result{
			Output: "Feedback collection is not available for organizations with custom data retention policies.",
		}, nil
	}

	return command.Result{
		Output: "Could not submit feedback. Please try again later.",
	}, nil
}

func (c FeedbackCommand) fallbackResult(_ context.Context, _ command.Args) (command.Result, error) {
	logger.DebugCF("commands", "rendered feedback command fallback output", map[string]any{
		"feedback_submission_available": false,
	})
	return command.Result{
		Output: feedbackCommandFallback,
	}, nil
}
