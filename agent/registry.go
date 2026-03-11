package agent

import (
	"fmt"

	"github.com/dntatme/agents/tool"
)

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

// RegisterOrReplace upserts an agent, replacing any existing agent with the same name.
func (r *Registry) RegisterOrReplace(a *Agent) {
	r.agents[a.Name] = a
}

// Unregister removes an agent by name. No-op if the agent doesn't exist.
func (r *Registry) Unregister(name string) {
	delete(r.agents, name)
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

// Finalize validates that all handoff targets exist in the registry.
// Call after all agents are registered, before Orchestrate.
func (r *Registry) Finalize() error {
	for agentName, ag := range r.agents {
		if ag.Tools == nil {
			continue
		}
		t := ag.Tools.Lookup(tool.TransferToolName)
		if t == nil {
			continue
		}
		def := t.Definition()
		if def.Parameters == nil {
			continue
		}
		agentProp, ok := def.Parameters.Properties["agent_name"]
		if !ok || agentProp == nil {
			continue
		}
		for _, target := range agentProp.Enum {
			if r.agents[target] == nil {
				return fmt.Errorf("agent %q references unknown handoff target %q", agentName, target)
			}
		}
	}
	return nil
}
