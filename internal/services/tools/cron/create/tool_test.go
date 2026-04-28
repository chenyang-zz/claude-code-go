package create

import (
	"context"
	"strings"
	"testing"

	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	cronshared "github.com/sheepzhao/claude-code-go/internal/services/tools/cron/shared"
)

func newTestTool() *Tool {
	return NewTool(cronshared.NewStore())
}

func TestName(t *testing.T) {
	tool := newTestTool()
	if tool.Name() != Name {
		t.Errorf("expected Name %q, got %q", Name, tool.Name())
	}
}

func TestDescription(t *testing.T) {
	tool := newTestTool()
	desc := tool.Description()
	if desc == "" {
		t.Error("expected non-empty description")
	}
	if !strings.Contains(desc, "cron") {
		t.Errorf("expected description to contain 'cron', got %q", desc)
	}
}

func TestIsReadOnly(t *testing.T) {
	tool := newTestTool()
	if tool.IsReadOnly() {
		t.Error("expected IsReadOnly to return false")
	}
}

func TestIsConcurrencySafe(t *testing.T) {
	tool := newTestTool()
	if !tool.IsConcurrencySafe() {
		t.Error("expected IsConcurrencySafe to return true")
	}
}

func TestRequiresUserInteraction(t *testing.T) {
	tool := newTestTool()
	if !tool.RequiresUserInteraction() {
		t.Error("expected RequiresUserInteraction to return true")
	}
}

func TestInputSchema(t *testing.T) {
	tool := newTestTool()
	schema := tool.InputSchema()

	// cron field
	prop, ok := schema.Properties["cron"]
	if !ok {
		t.Error("expected 'cron' property in input schema")
	} else {
		if prop.Type != coretool.ValueKindString {
			t.Errorf("expected 'cron' type to be string, got %s", prop.Type)
		}
		if !prop.Required {
			t.Error("expected 'cron' to be required")
		}
	}

	// prompt field
	prop, ok = schema.Properties["prompt"]
	if !ok {
		t.Error("expected 'prompt' property in input schema")
	} else {
		if prop.Type != coretool.ValueKindString {
			t.Errorf("expected 'prompt' type to be string, got %s", prop.Type)
		}
		if !prop.Required {
			t.Error("expected 'prompt' to be required")
		}
	}
}

func TestInvoke(t *testing.T) {
	tool := newTestTool()

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{
			"cron":   "*/5 * * * *",
			"prompt": "check every 5 minutes",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected result error: %s", result.Error)
	}
	if result.Output == "" {
		t.Error("expected non-empty output")
	}
	if !strings.Contains(result.Output, "Scheduled") {
		t.Errorf("expected output to contain 'Scheduled', got %q", result.Output)
	}

	// Verify Meta contains the Output data.
	data, ok := result.Meta["data"]
	if !ok {
		t.Fatal("expected Meta to contain 'data' key")
	}
	output, ok := data.(Output)
	if !ok {
		t.Fatalf("expected Meta data to be of type Output, got %T", data)
	}
	if output.ID == "" {
		t.Error("expected non-empty ID")
	}
	if output.Cron != "*/5 * * * *" {
		t.Errorf("expected cron %q, got %q", "*/5 * * * *", output.Cron)
	}
}

func TestInvokeWithRecurringFalse(t *testing.T) {
	tool := newTestTool()

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{
			"cron":      "30 14 28 2 *",
			"prompt":    "one-time reminder",
			"recurring": false,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected result error: %s", result.Error)
	}
	if !strings.Contains(result.Output, "one-shot") {
		t.Errorf("expected output to contain 'one-shot', got %q", result.Output)
	}
}

func TestInvokeInvalidCron(t *testing.T) {
	tool := newTestTool()

	// Too few fields.
	_, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{
			"cron":   "*/5 * * *",
			"prompt": "bad cron",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Note: validateCron returns an error in Result.Error, not as a Go error.
}

func TestInvokeInvalidCronFields(t *testing.T) {
	tool := newTestTool()

	result, _ := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{
			"cron":   "*/5 * * *",
			"prompt": "bad cron",
		},
	})
	if result.Error == "" {
		t.Error("expected result error for invalid cron expression")
	}
}

func TestInvokeMaxJobs(t *testing.T) {
	store := cronshared.NewStore()
	// Fill up to MaxJobs.
	for i := 0; i < cronshared.MaxJobs; i++ {
		store.Create("* * * * *", "fill", false, false)
	}

	tool := NewTool(store)
	result, _ := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{
			"cron":   "* * * * *",
			"prompt": "should fail",
		},
	})
	if result.Error == "" {
		t.Error("expected result error when exceeding MaxJobs")
	}
	if !strings.Contains(result.Error, "too many") {
		t.Errorf("expected error about too many jobs, got %q", result.Error)
	}
}

func TestInvokeNilReceiver(t *testing.T) {
	var tool *Tool
	_, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{
			"cron":   "*/5 * * * *",
			"prompt": "test",
		},
	})
	if err == nil {
		t.Error("expected error for nil receiver")
	}
}
