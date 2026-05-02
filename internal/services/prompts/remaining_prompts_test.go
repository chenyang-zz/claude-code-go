package prompts

import (
	"context"
	"strings"
	"testing"
)

func TestNotebookEditPromptSection(t *testing.T) {
	s := NotebookEditPromptSection{}
	if got := s.Name(); got != "notebook_edit_prompt" {
		t.Errorf("Name() = %q, want %q", got, "notebook_edit_prompt")
	}
	if s.IsVolatile() {
		t.Error("IsVolatile() = true, want false")
	}
	content, err := s.Compute(context.Background())
	if err != nil {
		t.Fatalf("Compute() error = %v", err)
	}
	if strings.TrimSpace(content) == "" {
		t.Error("Compute() returned empty content")
	}
	for _, phrase := range []string{"Jupyter notebook", "cell_number", "0-indexed", "edit_mode"} {
		if !strings.Contains(content, phrase) {
			t.Errorf("Compute() missing expected phrase: %q", phrase)
		}
	}
}

func TestWorktreePromptSection(t *testing.T) {
	s := WorktreePromptSection{}
	if got := s.Name(); got != "worktree_prompt" {
		t.Errorf("Name() = %q, want %q", got, "worktree_prompt")
	}
	if s.IsVolatile() {
		t.Error("IsVolatile() = true, want false")
	}
	content, err := s.Compute(context.Background())
	if err != nil {
		t.Fatalf("Compute() error = %v", err)
	}
	if strings.TrimSpace(content) == "" {
		t.Error("Compute() returned empty content")
	}
	for _, phrase := range []string{"EnterWorktree", "ExitWorktree", "worktree", "git repository", "action"} {
		if !strings.Contains(content, phrase) {
			t.Errorf("Compute() missing expected phrase: %q", phrase)
		}
	}
}

func TestTodoV2PromptSection(t *testing.T) {
	s := TodoV2PromptSection{}
	if got := s.Name(); got != "todo_v2_prompt" {
		t.Errorf("Name() = %q, want %q", got, "todo_v2_prompt")
	}
	if s.IsVolatile() {
		t.Error("IsVolatile() = true, want false")
	}
	content, err := s.Compute(context.Background())
	if err != nil {
		t.Fatalf("Compute() error = %v", err)
	}
	if strings.TrimSpace(content) == "" {
		t.Error("Compute() returned empty content")
	}
	for _, phrase := range []string{"TaskCreate", "TaskGet", "TaskList", "TaskUpdate", "TaskStop", "pending", "in_progress", "completed"} {
		if !strings.Contains(content, phrase) {
			t.Errorf("Compute() missing expected phrase: %q", phrase)
		}
	}
}

func TestCronPromptSection(t *testing.T) {
	s := CronPromptSection{}
	if got := s.Name(); got != "cron_prompt" {
		t.Errorf("Name() = %q, want %q", got, "cron_prompt")
	}
	if s.IsVolatile() {
		t.Error("IsVolatile() = true, want false")
	}
	content, err := s.Compute(context.Background())
	if err != nil {
		t.Fatalf("Compute() error = %v", err)
	}
	if strings.TrimSpace(content) == "" {
		t.Error("Compute() returned empty content")
	}
	for _, phrase := range []string{"CronCreate", "CronDelete", "CronList", "cron", "recurring", "durable", "ScheduleWakeup", "delaySeconds", "prompt cache"} {
		if !strings.Contains(content, phrase) {
			t.Errorf("Compute() missing expected phrase: %q", phrase)
		}
	}
}

func TestTeamPromptSection(t *testing.T) {
	s := TeamPromptSection{}
	if got := s.Name(); got != "team_prompt" {
		t.Errorf("Name() = %q, want %q", got, "team_prompt")
	}
	if s.IsVolatile() {
		t.Error("IsVolatile() = true, want false")
	}
	content, err := s.Compute(context.Background())
	if err != nil {
		t.Fatalf("Compute() error = %v", err)
	}
	if strings.TrimSpace(content) == "" {
		t.Error("Compute() returned empty content")
	}
	for _, phrase := range []string{"TeamCreate", "TeamDelete", "team", "teammate", "task list"} {
		if !strings.Contains(content, phrase) {
			t.Errorf("Compute() missing expected phrase: %q", phrase)
		}
	}
}

func TestAskUserQuestionPromptSection(t *testing.T) {
	s := AskUserQuestionPromptSection{}
	if got := s.Name(); got != "ask_user_question_prompt" {
		t.Errorf("Name() = %q, want %q", got, "ask_user_question_prompt")
	}
	if s.IsVolatile() {
		t.Error("IsVolatile() = true, want false")
	}
	content, err := s.Compute(context.Background())
	if err != nil {
		t.Fatalf("Compute() error = %v", err)
	}
	if strings.TrimSpace(content) == "" {
		t.Error("Compute() returned empty content")
	}
	for _, phrase := range []string{"AskUserQuestion", "preview", "multiSelect", "plan mode"} {
		if !strings.Contains(content, phrase) {
			t.Errorf("Compute() missing expected phrase: %q", phrase)
		}
	}
}

func TestPlanModePromptSection(t *testing.T) {
	s := PlanModePromptSection{}
	if got := s.Name(); got != "plan_mode_prompt" {
		t.Errorf("Name() = %q, want %q", got, "plan_mode_prompt")
	}
	if s.IsVolatile() {
		t.Error("IsVolatile() = true, want false")
	}
	content, err := s.Compute(context.Background())
	if err != nil {
		t.Fatalf("Compute() error = %v", err)
	}
	if strings.TrimSpace(content) == "" {
		t.Error("Compute() returned empty content")
	}
	for _, phrase := range []string{"EnterPlanMode", "ExitPlanMode", "plan mode", "user approval", "AskUserQuestion"} {
		if !strings.Contains(content, phrase) {
			t.Errorf("Compute() missing expected phrase: %q", phrase)
		}
	}
}

func TestSendMessagePromptSection(t *testing.T) {
	s := SendMessagePromptSection{}
	if got := s.Name(); got != "send_message_prompt" {
		t.Errorf("Name() = %q, want %q", got, "send_message_prompt")
	}
	if s.IsVolatile() {
		t.Error("IsVolatile() = true, want false")
	}
	content, err := s.Compute(context.Background())
	if err != nil {
		t.Fatalf("Compute() error = %v", err)
	}
	if strings.TrimSpace(content) == "" {
		t.Error("Compute() returned empty content")
	}
	for _, phrase := range []string{"SendMessage", "teammate", "Broadcast", "shutdown_request"} {
		if !strings.Contains(content, phrase) {
			t.Errorf("Compute() missing expected phrase: %q", phrase)
		}
	}
}

func TestRemoteTriggerPromptSection(t *testing.T) {
	s := RemoteTriggerPromptSection{}
	if got := s.Name(); got != "remote_trigger_prompt" {
		t.Errorf("Name() = %q, want %q", got, "remote_trigger_prompt")
	}
	if s.IsVolatile() {
		t.Error("IsVolatile() = true, want false")
	}
	content, err := s.Compute(context.Background())
	if err != nil {
		t.Fatalf("Compute() error = %v", err)
	}
	if strings.TrimSpace(content) == "" {
		t.Error("Compute() returned empty content")
	}
	for _, phrase := range []string{"RemoteTrigger", "claude.ai", "list", "get", "create", "update", "run"} {
		if !strings.Contains(content, phrase) {
			t.Errorf("Compute() missing expected phrase: %q", phrase)
		}
	}
}

func TestWebSearchPromptSection(t *testing.T) {
	s := WebSearchPromptSection{}
	if got := s.Name(); got != "websearch_prompt" {
		t.Errorf("Name() = %q, want %q", got, "websearch_prompt")
	}
	if s.IsVolatile() {
		t.Error("IsVolatile() = true, want false")
	}
	content, err := s.Compute(context.Background())
	if err != nil {
		t.Fatalf("Compute() error = %v", err)
	}
	if strings.TrimSpace(content) == "" {
		t.Error("Compute() returned empty content")
	}
	for _, phrase := range []string{"WebSearch", "Sources:", "markdown hyperlinks", "search queries"} {
		if !strings.Contains(content, phrase) {
			t.Errorf("Compute() missing expected phrase: %q", phrase)
		}
	}
}

func TestSkillPromptSection(t *testing.T) {
	s := SkillPromptSection{}
	if got := s.Name(); got != "skill_prompt" {
		t.Errorf("Name() = %q, want %q", got, "skill_prompt")
	}
	if s.IsVolatile() {
		t.Error("IsVolatile() = true, want false")
	}
	content, err := s.Compute(context.Background())
	if err != nil {
		t.Fatalf("Compute() error = %v", err)
	}
	if strings.TrimSpace(content) == "" {
		t.Error("Compute() returned empty content")
	}
	for _, phrase := range []string{"Skill", "slash command", "invoke", "BLOCKING REQUIREMENT"} {
		if !strings.Contains(content, phrase) {
			t.Errorf("Compute() missing expected phrase: %q", phrase)
		}
	}
}

func TestToolSearchPromptSection(t *testing.T) {
	s := ToolSearchPromptSection{}
	if got := s.Name(); got != "tool_search_prompt" {
		t.Errorf("Name() = %q, want %q", got, "tool_search_prompt")
	}
	if s.IsVolatile() {
		t.Error("IsVolatile() = true, want false")
	}
	content, err := s.Compute(context.Background())
	if err != nil {
		t.Fatalf("Compute() error = %v", err)
	}
	if strings.TrimSpace(content) == "" {
		t.Error("Compute() returned empty content")
	}
	for _, phrase := range []string{"ToolSearch", "deferred", "schema", "JSONSchema", "functions"} {
		if !strings.Contains(content, phrase) {
			t.Errorf("Compute() missing expected phrase: %q", phrase)
		}
	}
}

func TestMCPResourcePromptSection(t *testing.T) {
	s := MCPResourcePromptSection{}
	if got := s.Name(); got != "mcp_resource_prompt" {
		t.Errorf("Name() = %q, want %q", got, "mcp_resource_prompt")
	}
	if s.IsVolatile() {
		t.Error("IsVolatile() = true, want false")
	}
	content, err := s.Compute(context.Background())
	if err != nil {
		t.Fatalf("Compute() error = %v", err)
	}
	if strings.TrimSpace(content) == "" {
		t.Error("Compute() returned empty content")
	}
	for _, phrase := range []string{"ListMcpResources", "ReadMcpResource", "MCP", "server", "uri"} {
		if !strings.Contains(content, phrase) {
			t.Errorf("Compute() missing expected phrase: %q", phrase)
		}
	}
}

func TestLSPPromptSection(t *testing.T) {
	s := LSPPromptSection{}
	if got := s.Name(); got != "lsp_prompt" {
		t.Errorf("Name() = %q, want %q", got, "lsp_prompt")
	}
	if s.IsVolatile() {
		t.Error("IsVolatile() = true, want false")
	}
	content, err := s.Compute(context.Background())
	if err != nil {
		t.Fatalf("Compute() error = %v", err)
	}
	if strings.TrimSpace(content) == "" {
		t.Error("Compute() returned empty content")
	}
	for _, phrase := range []string{"LSP", "goToDefinition", "findReferences", "hover", "filePath"} {
		if !strings.Contains(content, phrase) {
			t.Errorf("Compute() missing expected phrase: %q", phrase)
		}
	}
}

func TestConfigPromptSection(t *testing.T) {
	s := ConfigPromptSection{}
	if got := s.Name(); got != "config_prompt" {
		t.Errorf("Name() = %q, want %q", got, "config_prompt")
	}
	if s.IsVolatile() {
		t.Error("IsVolatile() = true, want false")
	}
	content, err := s.Compute(context.Background())
	if err != nil {
		t.Fatalf("Compute() error = %v", err)
	}
	if strings.TrimSpace(content) == "" {
		t.Error("Compute() returned empty content")
	}
	for _, phrase := range []string{"ConfigTool", "settings", "theme", "model", "permission"} {
		if !strings.Contains(content, phrase) {
			t.Errorf("Compute() missing expected phrase: %q", phrase)
		}
	}
}

func TestBriefToolPromptSection(t *testing.T) {
	s := BriefToolPromptSection{}
	if got := s.Name(); got != "brief_tool_prompt" {
		t.Errorf("Name() = %q, want %q", got, "brief_tool_prompt")
	}
	if s.IsVolatile() {
		t.Error("IsVolatile() = true, want false")
	}
	content, err := s.Compute(context.Background())
	if err != nil {
		t.Fatalf("Compute() error = %v", err)
	}
	if strings.TrimSpace(content) == "" {
		t.Error("Compute() returned empty content")
	}
	for _, phrase := range []string{"SendUserMessage", "user", "markdown", "attachments", "status"} {
		if !strings.Contains(content, phrase) {
			t.Errorf("Compute() missing expected phrase: %q", phrase)
		}
	}
}

func TestSleepPromptSection(t *testing.T) {
	s := SleepPromptSection{}
	if got := s.Name(); got != "sleep_prompt" {
		t.Errorf("Name() = %q, want %q", got, "sleep_prompt")
	}
	if s.IsVolatile() {
		t.Error("IsVolatile() = true, want false")
	}
	content, err := s.Compute(context.Background())
	if err != nil {
		t.Fatalf("Compute() error = %v", err)
	}
	if strings.TrimSpace(content) == "" {
		t.Error("Compute() returned empty content")
	}
	for _, phrase := range []string{"Sleep", "duration", "interrupt", "concurrently"} {
		if !strings.Contains(content, phrase) {
			t.Errorf("Compute() missing expected phrase: %q", phrase)
		}
	}
}

func TestTodoWritePromptSection(t *testing.T) {
	s := TodoWritePromptSection{}
	if got := s.Name(); got != "todo_write_prompt" {
		t.Errorf("Name() = %q, want %q", got, "todo_write_prompt")
	}
	if s.IsVolatile() {
		t.Error("IsVolatile() = true, want false")
	}
	content, err := s.Compute(context.Background())
	if err != nil {
		t.Fatalf("Compute() error = %v", err)
	}
	if strings.TrimSpace(content) == "" {
		t.Error("Compute() returned empty content")
	}
	for _, phrase := range []string{"TodoWrite", "pending", "in_progress", "completed", "activeForm"} {
		if !strings.Contains(content, phrase) {
			t.Errorf("Compute() missing expected phrase: %q", phrase)
		}
	}
}
