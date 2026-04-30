package bundled

import (
	"context"
	"strings"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/internal/services/tools/skill"
)

func TestScheduleRemoteAgentsSkill(t *testing.T) {
	skill.ClearBundledSkills()
	registerScheduleRemoteAgentsSkill()

	skills := skill.GetBundledSkills()
	if len(skills) != 1 {
		t.Fatalf("expected 1 bundled skill, got %d", len(skills))
	}

	s := skills[0]
	if s.Metadata().Name != "schedule" {
		t.Errorf("expected name 'schedule', got %q", s.Metadata().Name)
	}

	result, err := s.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if !strings.Contains(result.Output, "Schedule Remote Agents") {
		t.Error("expected output to contain Schedule Remote Agents")
	}
	if !strings.Contains(result.Output, "RemoteTrigger") {
		t.Error("expected output to reference RemoteTrigger tool")
	}
	if !strings.Contains(result.Output, "cron") {
		t.Error("expected output to contain cron schedule information")
	}

	result, err = s.Execute(context.Background(), command.Args{RawLine: "check the deploy daily at 9am"})
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if !strings.Contains(result.Output, "check the deploy daily at 9am") {
		t.Error("expected output to contain user request")
	}
}
