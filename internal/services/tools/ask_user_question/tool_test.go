package ask_user_question

import (
	"context"
	"strings"
	"testing"

	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
)

func TestName(t *testing.T) {
	tool := NewTool()
	if tool.Name() != Name {
		t.Errorf("expected Name %q, got %q", Name, tool.Name())
	}
}

func TestDescription(t *testing.T) {
	tool := NewTool()
	if !strings.Contains(tool.Description(), "multiple choice questions") {
		t.Errorf("expected Description to contain 'multiple choice questions', got %q", tool.Description())
	}
}

func TestIsReadOnly(t *testing.T) {
	tool := NewTool()
	if !tool.IsReadOnly() {
		t.Error("expected IsReadOnly to return true")
	}
}

func TestIsConcurrencySafe(t *testing.T) {
	tool := NewTool()
	if !tool.IsConcurrencySafe() {
		t.Error("expected IsConcurrencySafe to return true")
	}
}

func TestRequiresUserInteraction(t *testing.T) {
	tool := NewTool()
	if !tool.RequiresUserInteraction() {
		t.Error("expected RequiresUserInteraction to return true")
	}
}

func TestInputSchema(t *testing.T) {
	tool := NewTool()
	schema := tool.InputSchema()

	if _, ok := schema.Properties["questions"]; !ok {
		t.Error("expected 'questions' property in input schema")
	}
	if schema.Properties["questions"].Required != true {
		t.Error("expected 'questions' to be required")
	}
	if schema.Properties["questions"].Type != coretool.ValueKindArray {
		t.Errorf("expected 'questions' type to be array, got %s", schema.Properties["questions"].Type)
	}

	if _, ok := schema.Properties["answers"]; !ok {
		t.Error("expected 'answers' property in input schema")
	}

	if _, ok := schema.Properties["annotations"]; !ok {
		t.Error("expected 'annotations' property in input schema")
	}

	if _, ok := schema.Properties["metadata"]; !ok {
		t.Error("expected 'metadata' property in input schema")
	}
}

func TestInvokeMinimalValidInput(t *testing.T) {
	tool := NewTool()
	input := map[string]any{
		"questions": []any{
			map[string]any{
				"question":    "Which library should we use?",
				"header":      "Library",
				"options": []any{
					map[string]any{
						"label":       "Option A",
						"description": "The first option",
					},
					map[string]any{
						"label":       "Option B",
						"description": "The second option",
					},
				},
				"multiSelect": false,
			},
		},
	}

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Name:  Name,
		Input: input,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected result error: %s", result.Error)
	}
	if result.Output == "" {
		t.Error("expected non-empty output text")
	}
}

func TestInvokeWithAnswers(t *testing.T) {
	tool := NewTool()
	input := map[string]any{
		"questions": []any{
			map[string]any{
				"question":    "Which library?",
				"header":      "Library",
				"options": []any{
					map[string]any{
						"label":       "A",
						"description": "Option A",
					},
					map[string]any{
						"label":       "B",
						"description": "Option B",
					},
				},
				"multiSelect": false,
			},
		},
		"answers": map[string]any{
			"Which library?": "A",
		},
	}

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Name:  Name,
		Input: input,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected result error: %s", result.Error)
	}
	if !strings.Contains(result.Output, `"Which library?"="A"`) {
		t.Errorf("expected output to contain the answer, got %q", result.Output)
	}

	// Verify Meta contains the Output data.
	if _, ok := result.Meta["data"]; !ok {
		t.Error("expected Meta to contain 'data' key")
	}
}

func TestInvokeEmptyQuestions(t *testing.T) {
	tool := NewTool()
	input := map[string]any{
		"questions": []any{},
	}

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Name:  Name,
		Input: input,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Error("expected error for empty questions")
	}
}

func TestInvokeTooManyQuestions(t *testing.T) {
	tool := NewTool()
	questions := make([]any, 5)
	for i := range 5 {
		questions[i] = map[string]any{
			"question":    "Q",
			"header":      "H",
			"options": []any{
				map[string]any{"label": "A", "description": "Desc A"},
				map[string]any{"label": "B", "description": "Desc B"},
			},
			"multiSelect": false,
		}
	}
	input := map[string]any{
		"questions": questions,
	}

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Name:  Name,
		Input: input,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Error("expected error for too many questions")
	}
}

func TestInvokeDuplicateQuestionText(t *testing.T) {
	tool := NewTool()
	input := map[string]any{
		"questions": []any{
			map[string]any{
				"question":    "Same question?",
				"header":      "Q1",
				"options": []any{
					map[string]any{"label": "A", "description": "Desc"},
					map[string]any{"label": "B", "description": "Desc"},
				},
				"multiSelect": false,
			},
			map[string]any{
				"question":    "Same question?",
				"header":      "Q2",
				"options": []any{
					map[string]any{"label": "C", "description": "Desc"},
					map[string]any{"label": "D", "description": "Desc"},
				},
				"multiSelect": false,
			},
		},
	}

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Name:  Name,
		Input: input,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Error("expected error for duplicate question text")
	}
}

func TestInvokeDuplicateOptionLabels(t *testing.T) {
	tool := NewTool()
	input := map[string]any{
		"questions": []any{
			map[string]any{
				"question":    "Pick one?",
				"header":      "Pick",
				"options": []any{
					map[string]any{"label": "Same", "description": "First"},
					map[string]any{"label": "Same", "description": "Second"},
				},
				"multiSelect": false,
			},
		},
	}

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Name:  Name,
		Input: input,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Error("expected error for duplicate option labels")
	}
}

func TestInvokeTooFewOptions(t *testing.T) {
	tool := NewTool()
	input := map[string]any{
		"questions": []any{
			map[string]any{
				"question":    "Pick?",
				"header":      "H",
				"options": []any{
					map[string]any{"label": "Only", "description": "Only one"},
				},
				"multiSelect": false,
			},
		},
	}

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Name:  Name,
		Input: input,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Error("expected error for too few options")
	}
}

func TestInvokeTooManyOptions(t *testing.T) {
	tool := NewTool()
	input := map[string]any{
		"questions": []any{
			map[string]any{
				"question":    "Pick?",
				"header":      "H",
				"options": []any{
					map[string]any{"label": "A", "description": "A"},
					map[string]any{"label": "B", "description": "B"},
					map[string]any{"label": "C", "description": "C"},
					map[string]any{"label": "D", "description": "D"},
					map[string]any{"label": "E", "description": "E"},
				},
				"multiSelect": false,
			},
		},
	}

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Name:  Name,
		Input: input,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Error("expected error for too many options")
	}
}

func TestInvokeHeaderTooLong(t *testing.T) {
	tool := NewTool()
	input := map[string]any{
		"questions": []any{
			map[string]any{
				"question":    "Q?",
				"header":      "This header is way too long",
				"options": []any{
					map[string]any{"label": "A", "description": "A"},
					map[string]any{"label": "B", "description": "B"},
				},
				"multiSelect": false,
			},
		},
	}

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Name:  Name,
		Input: input,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Error("expected error for header too long")
	}
}

func TestInvokeMissingQuestionText(t *testing.T) {
	tool := NewTool()
	input := map[string]any{
		"questions": []any{
			map[string]any{
				"question":    "",
				"header":      "H",
				"options": []any{
					map[string]any{"label": "A", "description": "A"},
					map[string]any{"label": "B", "description": "B"},
				},
				"multiSelect": false,
			},
		},
	}

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Name:  Name,
		Input: input,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Error("expected error for missing question text")
	}
}

func TestInvokeWithAnnotations(t *testing.T) {
	tool := NewTool()
	input := map[string]any{
		"questions": []any{
			map[string]any{
				"question":    "Which approach?",
				"header":      "Approach",
				"options": []any{
					map[string]any{"label": "A", "description": "Option A", "preview": "# Approach A\nDetails here"},
					map[string]any{"label": "B", "description": "Option B"},
				},
				"multiSelect": false,
			},
		},
		"answers": map[string]any{
			"Which approach?": "A",
		},
		"annotations": map[string]any{
			"Which approach?": map[string]any{
				"preview": "# Approach A\nDetails here",
				"notes":   "Looks good",
			},
		},
	}

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Name:  Name,
		Input: input,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected result error: %s", result.Error)
	}

	// Verify Meta contains the Output with annotations.
	data, ok := result.Meta["data"].(Output)
	if !ok {
		t.Fatal("expected Meta data to be of type Output")
	}
	if data.Annotations == nil {
		t.Error("expected annotations in output")
	} else if ann, ok := data.Annotations["Which approach?"]; !ok || ann.Notes != "Looks good" {
		t.Errorf("expected annotation notes 'Looks good', got %+v", data.Annotations)
	}
}

func TestInvokeNoAnswers(t *testing.T) {
	tool := NewTool()
	input := map[string]any{
		"questions": []any{
			map[string]any{
				"question":    "Q?",
				"header":      "H",
				"options": []any{
					map[string]any{"label": "A", "description": "A"},
					map[string]any{"label": "B", "description": "B"},
				},
				"multiSelect": false,
			},
		},
	}

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Name:  Name,
		Input: input,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected result error: %s", result.Error)
	}
	if !strings.Contains(result.Output, "No answers were provided") {
		t.Errorf("expected 'No answers were provided', got %q", result.Output)
	}
}

func TestInvokeNilReceiver(t *testing.T) {
	var tool *Tool
	_, err := tool.Invoke(context.Background(), coretool.Call{
		Name:  Name,
		Input: map[string]any{},
	})
	if err == nil {
		t.Error("expected error for nil receiver")
	}
}
