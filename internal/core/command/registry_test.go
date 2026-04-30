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
	second := stubCommand{meta: Metadata{Name: "resume", Aliases: []string{"continue"}, Description: "resume session"}}

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
	got, ok = registry.Get("/continue")
	if !ok {
		t.Fatal("Get(/continue) ok = false, want true")
	}
	if got.Metadata().Name != "resume" {
		t.Fatalf("Get(/continue) name = %q, want resume", got.Metadata().Name)
	}

	list := registry.List()
	if len(list) != 2 {
		t.Fatalf("List() len = %d, want 2", len(list))
	}
	if list[0].Metadata().Name != "help" || list[1].Metadata().Name != "resume" {
		t.Fatalf("List() order = %#v, want help then resume", []string{list[0].Metadata().Name, list[1].Metadata().Name})
	}
}

// TestInMemoryRegistryUnregister removes a command and its aliases.
func TestInMemoryRegistryUnregister(t *testing.T) {
	registry := NewInMemoryRegistry()

	first := stubCommand{meta: Metadata{Name: "help", Description: "show help"}}
	second := stubCommand{meta: Metadata{Name: "resume", Aliases: []string{"continue"}, Description: "resume session"}}

	if err := registry.Register(first); err != nil {
		t.Fatalf("Register(first) error = %v", err)
	}
	if err := registry.Register(second); err != nil {
		t.Fatalf("Register(second) error = %v", err)
	}

	if err := registry.Unregister("resume"); err != nil {
		t.Fatalf("Unregister(resume) error = %v", err)
	}

	if _, ok := registry.Get("resume"); ok {
		t.Fatal("Get(resume) ok = true, want false after unregister")
	}
	if _, ok := registry.Get("continue"); ok {
		t.Fatal("Get(continue) ok = true, want false after unregister")
	}
	if _, ok := registry.Get("help"); !ok {
		t.Fatal("Get(help) ok = false, want true")
	}

	list := registry.List()
	if len(list) != 1 {
		t.Fatalf("List() len = %d, want 1", len(list))
	}

	if err := registry.Unregister("nonexistent"); err == nil {
		t.Fatal("Unregister(nonexistent) error = nil, want error")
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
	if err := registry.Register(stubCommand{meta: Metadata{Name: "resume", Aliases: []string{"help"}}}); err == nil {
		t.Fatal("Register(alias collides with existing command) error = nil, want error")
	}
}
