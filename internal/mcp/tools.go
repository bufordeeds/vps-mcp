// Package mcp implements a minimal Model Context Protocol server.
//
// It speaks JSON-RPC 2.0 over a chosen transport (currently stdio) and
// supports the three methods needed for tool servers: initialize,
// tools/list, and tools/call.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

// Tool is the interface every callable tool must implement.
type Tool interface {
	// Name is the unique identifier used by tools/call.
	Name() string

	// Description is shown to the LLM so it can decide when to invoke the tool.
	Description() string

	// InputSchema returns a JSON Schema describing the tool's arguments.
	InputSchema() json.RawMessage

	// Call executes the tool. The returned text is sent back as the tool result.
	Call(ctx context.Context, args json.RawMessage) (string, error)
}

// Registry holds the set of tools the server exposes.
type Registry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

// Register adds a tool. Panics on name collisions to surface programmer error early.
func (r *Registry) Register(t Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.tools[t.Name()]; exists {
		panic(fmt.Sprintf("mcp: duplicate tool %q", t.Name()))
	}
	r.tools[t.Name()] = t
}

// List returns all registered tools, sorted alphabetically by name.
func (r *Registry) List() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Tool, 0, len(r.tools))
	for _, t := range r.tools {
		out = append(out, t)
	}
	// Stable order so tools/list responses are deterministic.
	sortTools(out)
	return out
}

// Get returns the tool with the given name, or nil if no such tool is registered.
func (r *Registry) Get(name string) Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.tools[name]
}
