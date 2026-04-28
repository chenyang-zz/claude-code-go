package commands

import (
	"context"
	"reflect"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestOnboardingCommandMetadata verifies /onboarding is exposed as a hidden command.
func TestOnboardingCommandMetadata(t *testing.T) {
	meta := OnboardingCommand{}.Metadata()

	if !reflect.DeepEqual(meta, command.Metadata{
		Name:        "onboarding",
		Description: "Run internal onboarding flow",
		Usage:       "/onboarding",
		Hidden:      true,
	}) {
		t.Fatalf("Metadata() = %#v, want onboarding metadata", meta)
	}
}

// TestOnboardingCommandExecute verifies /onboarding returns the stable fallback for no-arg execution.
func TestOnboardingCommandExecute(t *testing.T) {
	result, err := OnboardingCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != onboardingCommandFallback {
		t.Fatalf("Execute() output = %q, want %q", result.Output, onboardingCommandFallback)
	}
}

// TestOnboardingCommandExecuteRejectsArgs verifies /onboarding accepts no arguments.
func TestOnboardingCommandExecuteRejectsArgs(t *testing.T) {
	_, err := OnboardingCommand{}.Execute(context.Background(), command.Args{RawLine: "force"})
	if err == nil {
		t.Fatal("Execute() error = nil, want usage error")
	}
	if err.Error() != "usage: /onboarding" {
		t.Fatalf("Execute() error = %q, want usage error", err.Error())
	}
}
