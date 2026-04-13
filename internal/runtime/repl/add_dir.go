package repl

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	servicecommands "github.com/sheepzhao/claude-code-go/internal/services/commands"
)

const (
	addDirPathPrompt            = "Enter a directory path to add, or press Enter to cancel."
	addDirChoiceHeading         = "Choose how to add this working directory:"
	addDirChoiceSession         = "1. Add for this session only"
	addDirChoiceRemember        = "2. Add and save to local settings"
	addDirChoicePrompt          = "Select 1 or 2, or press Enter to cancel."
	addDirChoiceInvalid         = "Invalid selection. Use 1 or 2, or press Enter to cancel."
	addDirCancelledMessage      = "Did not add a working directory."
	addDirCancelledPathTemplate = "Did not add %s as a working directory."
)

// runAddDirCommand handles the interactive `/add-dir` text flow before delegating persistence back into the shared command logic.
func (r *Runner) runAddDirCommand(ctx context.Context, cmd servicecommands.AddDirCommand, body string) error {
	requested := strings.TrimSpace(body)
	if requested == "" {
		if r == nil || r.Input == nil {
			return r.Renderer.RenderLine(fmt.Sprintf("usage: %s", cmd.Metadata().Usage))
		}
		if err := r.Renderer.RenderLine(addDirPathPrompt); err != nil {
			return err
		}
		line, ok, err := r.readInteractiveLine()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		if !ok {
			return r.Renderer.RenderLine(addDirCancelledMessage)
		}
		requested = line
	}

	absolutePath, err := cmd.ResolveDirectory(requested)
	if err != nil {
		return r.Renderer.RenderLine(err.Error())
	}

	destination, cancelled, err := r.readAddDirDestination(absolutePath)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}
	if cancelled {
		return r.Renderer.RenderLine(fmt.Sprintf(addDirCancelledPathTemplate, absolutePath))
	}
	if destination == "" {
		return nil
	}

	result, err := cmd.ApplyDirectory(ctx, absolutePath, destination)
	if err != nil {
		return err
	}
	return r.Renderer.RenderLine(result.Output)
}

func (r *Runner) readAddDirDestination(absolutePath string) (servicecommands.AddDirDestination, bool, error) {
	if r == nil || r.Input == nil {
		return servicecommands.AddDirDestinationLocalSettings, false, nil
	}

	lines := []string{
		fmt.Sprintf("Directory: %s", absolutePath),
		addDirChoiceHeading,
		addDirChoiceSession,
		addDirChoiceRemember,
		addDirChoicePrompt,
	}
	if err := r.Renderer.RenderLine(strings.Join(lines, "\n")); err != nil {
		return "", false, err
	}

	line, ok, err := r.readInteractiveLine()
	if err != nil {
		return "", false, err
	}
	if !ok {
		return "", true, nil
	}

	switch line {
	case "1":
		return servicecommands.AddDirDestinationSession, false, nil
	case "2":
		return servicecommands.AddDirDestinationLocalSettings, false, nil
	default:
		if err := r.Renderer.RenderLine(addDirChoiceInvalid); err != nil {
			return "", false, err
		}
		return "", false, nil
	}
}

func (r *Runner) readInteractiveLine() (string, bool, error) {
	if r == nil || r.Input == nil {
		return "", false, io.EOF
	}

	if r.inputReader == nil || r.inputSource != r.Input {
		r.inputSource = r.Input
		r.inputReader = bufio.NewReader(r.Input)
	}

	line, err := r.inputReader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", false, err
	}
	if errors.Is(err, io.EOF) && line == "" {
		return "", false, io.EOF
	}

	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return "", false, nil
	}
	return trimmed, true, nil
}
