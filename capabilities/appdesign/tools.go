package appdesign

import (
	"context"
	"fmt"

	"github.com/kacperpaczos/agents/tool"
)

// AddComponentTool returns a tool that adds a component to the app design.
func AddComponentTool() tool.Tool {
	return tool.Func("add_component", "Add a component to the app design.").
		StringParam("name", "Unique component name.", true).
		StringParam("description", "What this component does.", true).
		Handler(func(_ context.Context, args map[string]any, state map[string]any) (map[string]any, error) {
			name, _ := args["name"].(string)
			desc, _ := args["description"].(string)
			d := GetDesign(state)
			for _, c := range d.Components {
				if c.Name == name {
					return map[string]any{"error": fmt.Sprintf("component %q already exists", name)}, nil
				}
			}
			d.Components = append(d.Components, Component{Name: name, Description: desc})
			return map[string]any{"status": "added", "component": name}, nil
		}).
		Build()
}

// RemoveComponentTool returns a tool that removes a component and its connections.
func RemoveComponentTool() tool.Tool {
	return tool.Func("remove_component", "Remove a component from the app design. Also removes any connections involving it.").
		StringParam("name", "Name of the component to remove.", true).
		Handler(func(_ context.Context, args map[string]any, state map[string]any) (map[string]any, error) {
			name, _ := args["name"].(string)
			d := GetDesign(state)
			found := false
			filtered := d.Components[:0]
			for _, c := range d.Components {
				if c.Name == name {
					found = true
					continue
				}
				filtered = append(filtered, c)
			}
			if !found {
				return map[string]any{"error": fmt.Sprintf("component %q not found", name)}, nil
			}
			d.Components = filtered
			var conns []Connection
			for _, c := range d.Connections {
				if c.From != name && c.To != name {
					conns = append(conns, c)
				}
			}
			d.Connections = conns
			return map[string]any{"status": "removed", "component": name}, nil
		}).
		Build()
}

// AddConnectionTool returns a tool that adds a directed connection between two components.
func AddConnectionTool() tool.Tool {
	return tool.Func("add_connection", "Add a directed connection between two components.").
		StringParam("from", "Source component name.", true).
		StringParam("to", "Target component name.", true).
		StringParam("description", "What data or control flows over this connection.", true).
		Handler(func(_ context.Context, args map[string]any, state map[string]any) (map[string]any, error) {
			from, _ := args["from"].(string)
			to, _ := args["to"].(string)
			desc, _ := args["description"].(string)
			d := GetDesign(state)
			hasFrom, hasTo := false, false
			for _, c := range d.Components {
				if c.Name == from {
					hasFrom = true
				}
				if c.Name == to {
					hasTo = true
				}
			}
			if !hasFrom {
				return map[string]any{"error": fmt.Sprintf("source component %q not found", from)}, nil
			}
			if !hasTo {
				return map[string]any{"error": fmt.Sprintf("target component %q not found", to)}, nil
			}
			d.Connections = append(d.Connections, Connection{From: from, To: to, Description: desc})
			return map[string]any{"status": "connected", "from": from, "to": to}, nil
		}).
		Build()
}

// RemoveConnectionTool returns a tool that removes a connection between two components.
func RemoveConnectionTool() tool.Tool {
	return tool.Func("remove_connection", "Remove a connection between two components.").
		StringParam("from", "Source component name.", true).
		StringParam("to", "Target component name.", true).
		Handler(func(_ context.Context, args map[string]any, state map[string]any) (map[string]any, error) {
			from, _ := args["from"].(string)
			to, _ := args["to"].(string)
			d := GetDesign(state)
			found := false
			var conns []Connection
			for _, c := range d.Connections {
				if c.From == from && c.To == to {
					found = true
					continue
				}
				conns = append(conns, c)
			}
			if !found {
				return map[string]any{"error": fmt.Sprintf("connection %s -> %s not found", from, to)}, nil
			}
			d.Connections = conns
			return map[string]any{"status": "removed", "from": from, "to": to}, nil
		}).
		Build()
}

// ListComponentsTool returns a tool that lists all components and connections.
func ListComponentsTool() tool.Tool {
	return tool.Func("list_components", "List all components and connections in the current app design.").
		NoParams().
		Handler(func(_ context.Context, _ map[string]any, state map[string]any) (map[string]any, error) {
			d := GetDesign(state)
			return map[string]any{"design": d.String()}, nil
		}).
		Build()
}

// RegisterAll registers all app design tools into the given registry.
func RegisterAll(r *tool.Registry) {
	r.Register(AddComponentTool())
	r.Register(RemoveComponentTool())
	r.Register(AddConnectionTool())
	r.Register(RemoveConnectionTool())
	r.Register(ListComponentsTool())
}
