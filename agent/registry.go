package agent

import "fmt"

// Registry stores agents by name.
type Registry struct {
	agents map[string]*Agent
}

// NewRegistry creates an empty agent registry.
func NewRegistry() *Registry {
	return &Registry{agents: make(map[string]*Agent)}
}

// Register adds an agent. Panics on duplicate names.
func (r *Registry) Register(a *Agent) {
	if _, exists := r.agents[a.Name]; exists {
		panic(fmt.Sprintf("agent %q already registered", a.Name))
	}
	r.agents[a.Name] = a
}

// Lookup returns an agent by name, or nil if not found.
func (r *Registry) Lookup(name string) *Agent {
	return r.agents[name]
}

// Names returns all registered agent names.
func (r *Registry) Names() []string {
	out := make([]string, 0, len(r.agents))
	for name := range r.agents {
		out = append(out, name)
	}
	return out
}
