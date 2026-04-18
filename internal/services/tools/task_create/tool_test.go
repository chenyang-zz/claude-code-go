package task_create

import (
	"context"
	"testing"

	coretask "github.com/sheepzhao/claude-code-go/internal/core/task"
	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
)

// mockCreateStore is a minimal fake implementing TaskCreator.
type mockCreateStore struct {
	created coretask.NewTask
	id      string
	err     error
}

func (m *mockCreateStore) Create(_ context.Context, data coretask.NewTask) (string, error) {
	m.created = data
	return m.id, m.err
}

func TestCreateTool_Invoke(t *testing.T) {
	store := &mockCreateStore{id: "1"}
	tool := NewTool(store)

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{
			"subject":     "Test task",
			"description": "A test description",
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result.Error != "" {
		t.Fatalf("Invoke() result.Error = %q", result.Error)
	}
	if store.created.Subject != "Test task" {
		t.Errorf("Subject = %q, want %q", store.created.Subject, "Test task")
	}
}

func TestCreateTool_MissingSubject(t *testing.T) {
	store := &mockCreateStore{}
	tool := NewTool(store)

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{
			"description": "No subject",
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result.Error == "" {
		t.Fatal("Expected error for missing subject")
	}
}

func TestCreateTool_MissingDescription(t *testing.T) {
	store := &mockCreateStore{}
	tool := NewTool(store)

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{
			"subject": "No desc",
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result.Error == "" {
		t.Fatal("Expected error for missing description")
	}
}

func TestCreateTool_NilReceiver(t *testing.T) {
	var tool *Tool
	_, err := tool.Invoke(context.Background(), coretool.Call{})
	if err == nil {
		t.Fatal("Expected error for nil receiver")
	}
}

func TestCreateTool_NilStore(t *testing.T) {
	tool := NewTool(nil)
	result, _ := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{"subject": "A", "description": "B"},
	})
	if result.Error == "" {
		t.Fatal("Expected error for nil store")
	}
}

func TestCreateTool_WithMetadata(t *testing.T) {
	store := &mockCreateStore{id: "5"}
	tool := NewTool(store)

	result, _ := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{
			"subject":     "Meta task",
			"description": "Has metadata",
			"metadata":    map[string]any{"key": "value"},
		},
	})
	if result.Error != "" {
		t.Fatalf("result.Error = %q", result.Error)
	}
	if store.created.Metadata["key"] != "value" {
		t.Errorf("Metadata[key] = %v, want %q", store.created.Metadata["key"], "value")
	}
}
