package commands

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/internal/core/message"
	coresession "github.com/sheepzhao/claude-code-go/internal/core/session"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const seedSessionsNotConfiguredMessage = "Seed command is not available because session storage is not configured."

// SeedSessionsCommand writes a fixed set of demo sessions into the configured session store for manual `/resume` verification.
type SeedSessionsCommand struct {
	// Repository persists the demo sessions into the active session database.
	Repository coresession.Repository
	// ProjectPath identifies the current workspace that should receive the primary demo sessions.
	ProjectPath string
	// Now supplies timestamps for deterministic tests.
	Now func() time.Time
}

// Metadata returns the canonical slash descriptor for /seed-sessions.
func (c SeedSessionsCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "seed-sessions",
		Description: "Insert demo persisted sessions for /resume testing",
		Usage:       "/seed-sessions",
	}
}

// Execute writes one deterministic demo dataset into the configured session repository.
func (c SeedSessionsCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = args

	if c.Repository == nil {
		return command.Result{Output: seedSessionsNotConfiguredMessage}, nil
	}

	now := c.now().UTC().Truncate(time.Minute)
	currentProject := normalizeSeedProjectPath(c.ProjectPath)
	otherProject := deriveOtherProjectPath(currentProject)
	sessions := []coresession.Session{
		{
			ID:          "seed-current-latest",
			ProjectPath: currentProject,
			Messages: []message.Message{
				{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("latest prompt about deploy verification")}},
			},
			UpdatedAt: now.Add(-10 * time.Minute),
		},
		{
			ID:          "seed-current-retro",
			ProjectPath: currentProject,
			CustomTitle: "Deploy retrospective",
			Messages: []message.Message{
				{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("notes from the last release")}},
			},
			UpdatedAt: now.Add(-20 * time.Minute),
		},
		{
			ID:          "seed-current-debug",
			ProjectPath: currentProject,
			Messages: []message.Message{
				{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("debug failing deploy job")}},
			},
			UpdatedAt: now.Add(-30 * time.Minute),
		},
		{
			ID:          "seed-other-project",
			ProjectPath: otherProject,
			CustomTitle: "Other repo deploy",
			Messages: []message.Message{
				{Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("deploy change from another project")}},
			},
			UpdatedAt: now.Add(-5 * time.Minute),
		},
	}
	for _, session := range sessions {
		if err := c.Repository.Save(ctx, session); err != nil {
			return command.Result{}, err
		}
	}

	logger.DebugCF("commands", "seeded demo sessions for resume testing", map[string]any{
		"count":            len(sessions),
		"project_path":     currentProject,
		"other_project":    otherProject,
		"repository_set":   c.Repository != nil,
		"latest_seeded_id": sessions[0].ID,
	})

	lines := []string{
		fmt.Sprintf("Seeded %d demo conversations into session storage.", len(sessions)),
		fmt.Sprintf("Current project: %s", currentProject),
		fmt.Sprintf("Other project: %s", otherProject),
		"Inserted sessions:",
	}
	for _, session := range sessions {
		label := session.ID
		if strings.TrimSpace(session.CustomTitle) != "" {
			label = fmt.Sprintf("%s (%s)", session.ID, session.CustomTitle)
		}
		lines = append(lines, "- "+label)
	}
	lines = append(lines,
		"Try these next:",
		"  cc /resume",
		"  cc /resume deploy",
		"  cc /resume seed-current-latest hello",
	)

	return command.Result{Output: strings.Join(lines, "\n")}, nil
}

func (c SeedSessionsCommand) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

func normalizeSeedProjectPath(projectPath string) string {
	trimmed := strings.TrimSpace(projectPath)
	if trimmed == "" {
		return "/tmp/claude-code-go-demo"
	}
	return filepath.Clean(trimmed)
}

func deriveOtherProjectPath(projectPath string) string {
	base := filepath.Base(projectPath)
	if base == "." || base == string(filepath.Separator) || base == "" {
		base = "claude-code-go-demo"
	}
	return filepath.Join(filepath.Dir(projectPath), base+"-other")
}
