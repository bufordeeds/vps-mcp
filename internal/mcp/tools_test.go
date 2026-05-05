package mcp

import (
	"context"
	"encoding/json"
	"testing"
)

type fakeTool struct {
	name string
}

func (f *fakeTool) Name() string                 { return f.name }
func (f *fakeTool) Description() string          { return "fake" }
func (f *fakeTool) InputSchema() json.RawMessage { return json.RawMessage(`{"type":"object"}`) }
func (f *fakeTool) Call(context.Context, json.RawMessage) (string, error) {
	return "ok", nil
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := NewRegistry()
	r.Register(&fakeTool{name: "alpha"})
	r.Register(&fakeTool{name: "beta"})

	if got := r.Get("alpha"); got == nil || got.Name() != "alpha" {
		t.Fatalf("Get(alpha) = %v, want alpha tool", got)
	}
	if got := r.Get("missing"); got != nil {
		t.Fatalf("Get(missing) = %v, want nil", got)
	}
}

func TestRegistry_ListIsSorted(t *testing.T) {
	r := NewRegistry()
	r.Register(&fakeTool{name: "charlie"})
	r.Register(&fakeTool{name: "alpha"})
	r.Register(&fakeTool{name: "bravo"})

	got := r.List()
	want := []string{"alpha", "bravo", "charlie"}
	if len(got) != len(want) {
		t.Fatalf("List() len = %d, want %d", len(got), len(want))
	}
	for i, name := range want {
		if got[i].Name() != name {
			t.Errorf("List()[%d] = %q, want %q", i, got[i].Name(), name)
		}
	}
}

func TestRegistry_DuplicatePanics(t *testing.T) {
	r := NewRegistry()
	r.Register(&fakeTool{name: "x"})

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on duplicate registration")
		}
	}()
	r.Register(&fakeTool{name: "x"})
}
