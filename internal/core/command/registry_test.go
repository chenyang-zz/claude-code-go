package command

import (
	"context"
	"testing"
)

type stubCommand struct {
	meta Metadata
}

func (c stubCommand) Metadata() Metadata {
	return c.meta
}

func (c stubCommand) Execute(ctx context.Context, args Args) (Result, error) {
	_ = ctx
	_ = args
	return Result{}, nil
}

// TestInMemoryRegistryRegisterAndLookup verifies registry registration, normalization and stable list order.
func TestInMemoryRegistryRegisterAndLookup(t *testing.T) {
	registry := NewInMemoryRegistry()

	first := stubCommand{meta: Metadata{Name: "help", Description: "show help"}}
	second := stubCommand{meta: Metadata{Name: "resume", Description: "resume session"}}

	if err := registry.Register(first); err != nil {
		t.Fatalf("Register(first) error = %v", err)
	}
	if err := registry.Register(second); err != nil {
		t.Fatalf("Register(second) error = %v", err)
	}

	got, ok := registry.Get("/HELP")
	if !ok {
		t.Fatal("Get(/HELP) ok = false, want true")
	}
	if got.Metadata().Name != "help" {
		t.Fatalf("Get(/HELP) name = %q, want help", got.Metadata().Name)
	}

	list := registry.List()
	if len(list) != 2 {
		t.Fatalf("List() len = %d, want 2", len(list))
	}
	if list[0].Metadata().Name != "help" || list[1].Metadata().Name != "resume" {
		t.Fatalf("List() order = %#v, want help then resume", []string{list[0].Metadata().Name, list[1].Metadata().Name})
	}
}

// TestInMemoryRegistryRejectsInvalidCommands verifies empty names and duplicates are rejected.
func TestInMemoryRegistryRejectsInvalidCommands(t *testing.T) {
	registry := NewInMemoryRegistry()

	if err := registry.Register(stubCommand{}); err == nil {
		t.Fatal("Register(empty name) error = nil, want error")
	}
	if err := registry.Register(stubCommand{meta: Metadata{Name: "help"}}); err != nil {
		t.Fatalf("Register(help) error = %v", err)
	}
	if err := registry.Register(stubCommand{meta: Metadata{Name: "/help"}}); err == nil {
		t.Fatal("Register(duplicate help) error = nil, want error")
	}
}
