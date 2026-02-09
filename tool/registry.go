package tool

import (
	"fmt"

	"google.golang.org/genai"
)

// Registry holds named tools and produces Gemini-compatible definitions.
type Registry struct {
	tools map[string]Tool
	order []string // preserves insertion order
}

// NewRegistry creates an empty tool registry.
func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

// Register adds a tool. Panics on duplicate names.
func (r *Registry) Register(t Tool) {
	name := t.Definition().Name
	if _, exists := r.tools[name]; exists {
		panic(fmt.Sprintf("tool %q already registered", name))
	}
	r.tools[name] = t
	r.order = append(r.order, name)
}

// Lookup returns a tool by name, or nil if not found.
func (r *Registry) Lookup(name string) Tool {
	return r.tools[name]
}

// Definitions returns all function declarations for Gemini config.
func (r *Registry) Definitions() []*genai.FunctionDeclaration {
	defs := make([]*genai.FunctionDeclaration, 0, len(r.order))
	for _, name := range r.order {
		defs = append(defs, r.tools[name].Definition())
	}
	return defs
}

// Names returns all registered tool names.
func (r *Registry) Names() []string {
	out := make([]string, len(r.order))
	copy(out, r.order)
	return out
}

// Len returns the number of registered tools.
func (r *Registry) Len() int {
	return len(r.tools)
}
