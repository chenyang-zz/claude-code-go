package agent

import (
	"strings"
	"testing"
)

func TestInMemoryRegistry_RegisterAndGet(t *testing.T) {
	reg := NewInMemoryRegistry()

	def := Definition{AgentType: "explore", Source: "built-in"}
	if err := reg.Register(def); err != nil {
		t.Fatalf("Register() unexpected error: %v", err)
	}

	got, ok := reg.Get("explore")
	if !ok {
		t.Fatal("Get() expected ok=true, got false")
	}
	if got.AgentType != def.AgentType || got.Source != def.Source {
		t.Errorf("Get() = %+v, want %+v", got, def)
	}
}

func TestInMemoryRegistry_RegisterDuplicate(t *testing.T) {
	reg := NewInMemoryRegistry()

	def := Definition{AgentType: "explore", Source: "built-in"}
	if err := reg.Register(def); err != nil {
		t.Fatalf("first Register() unexpected error: %v", err)
	}

	err := reg.Register(def)
	if err == nil {
		t.Fatal("Register() duplicate expected error, got nil")
	}
	if !strings.Contains(err.Error(), "already registered") {
		t.Errorf("Register() error = %q, want containing 'already registered'", err.Error())
	}
}

func TestInMemoryRegistry_RegisterEmptyAgentType(t *testing.T) {
	reg := NewInMemoryRegistry()

	err := reg.Register(Definition{AgentType: ""})
	if err == nil {
		t.Fatal("Register() empty agent type expected error, got nil")
	}
	if !strings.Contains(err.Error(), "agent type is empty") {
		t.Errorf("Register() error = %q, want containing 'agent type is empty'", err.Error())
	}
}

func TestInMemoryRegistry_GetNotFound(t *testing.T) {
	reg := NewInMemoryRegistry()

	_, ok := reg.Get("nonexistent")
	if ok {
		t.Error("Get() expected ok=false for nonexistent agent")
	}
}

func TestInMemoryRegistry_List(t *testing.T) {
	reg := NewInMemoryRegistry()

	defs := []Definition{
		{AgentType: "alpha", Source: "built-in"},
		{AgentType: "beta", Source: "built-in"},
		{AgentType: "gamma", Source: "custom"},
	}

	for _, d := range defs {
		if err := reg.Register(d); err != nil {
			t.Fatalf("Register(%q) unexpected error: %v", d.AgentType, err)
		}
	}

	list := reg.List()
	if len(list) != 3 {
		t.Fatalf("List() len = %d, want 3", len(list))
	}
	if list[0].AgentType != "alpha" {
		t.Errorf("List()[0].AgentType = %q, want 'alpha'", list[0].AgentType)
	}
	if list[1].AgentType != "beta" {
		t.Errorf("List()[1].AgentType = %q, want 'beta'", list[1].AgentType)
	}
	if list[2].AgentType != "gamma" {
		t.Errorf("List()[2].AgentType = %q, want 'gamma'", list[2].AgentType)
	}
}

func TestInMemoryRegistry_ListEmpty(t *testing.T) {
	reg := NewInMemoryRegistry()

	list := reg.List()
	if len(list) != 0 {
		t.Errorf("List() len = %d, want 0", len(list))
	}
}

func TestInMemoryRegistry_Remove(t *testing.T) {
	reg := NewInMemoryRegistry()

	def := Definition{AgentType: "explore", Source: "built-in"}
	if err := reg.Register(def); err != nil {
		t.Fatalf("Register() unexpected error: %v", err)
	}

	removed := reg.Remove("explore")
	if !removed {
		t.Error("Remove() expected true, got false")
	}

	_, ok := reg.Get("explore")
	if ok {
		t.Error("Get() after Remove expected ok=false")
	}
}

func TestInMemoryRegistry_RemoveNotFound(t *testing.T) {
	reg := NewInMemoryRegistry()

	removed := reg.Remove("nonexistent")
	if removed {
		t.Error("Remove() expected false for nonexistent agent")
	}
}

func TestInMemoryRegistry_RemoveMaintainsOrder(t *testing.T) {
	reg := NewInMemoryRegistry()

	defs := []Definition{
		{AgentType: "alpha", Source: "built-in"},
		{AgentType: "beta", Source: "built-in"},
		{AgentType: "gamma", Source: "built-in"},
	}

	for _, d := range defs {
		if err := reg.Register(d); err != nil {
			t.Fatalf("Register(%q) unexpected error: %v", d.AgentType, err)
		}
	}

	reg.Remove("beta")

	list := reg.List()
	if len(list) != 2 {
		t.Fatalf("List() after Remove len = %d, want 2", len(list))
	}
	if list[0].AgentType != "alpha" {
		t.Errorf("List()[0].AgentType = %q, want 'alpha'", list[0].AgentType)
	}
	if list[1].AgentType != "gamma" {
		t.Errorf("List()[1].AgentType = %q, want 'gamma'", list[1].AgentType)
	}
}

func TestInMemoryRegistry_NilRegistry(t *testing.T) {
	var reg *InMemoryRegistry

	err := reg.Register(Definition{AgentType: "x"})
	if err == nil {
		t.Error("Register() on nil registry expected error")
	}

	_, ok := reg.Get("x")
	if ok {
		t.Error("Get() on nil registry expected ok=false")
	}

	list := reg.List()
	if list != nil {
		t.Errorf("List() on nil registry = %v, want nil", list)
	}

	removed := reg.Remove("x")
	if removed {
		t.Error("Remove() on nil registry expected false")
	}
}
