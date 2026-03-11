package tool

import (
	"context"
	"fmt"

	"github.com/dntatme/agents/llm"
)

const EndTurnToolName = "end_turn"

// NewEndTurnTool creates the end_turn tool definition.
// Agents call this to signal their turn is complete (done or idle).
// The reactor intercepts this call — the handler is never executed.
func NewEndTurnTool() Tool {
	return &FuncTool{
		Decl: &llm.FunctionDeclaration{
			Name:        EndTurnToolName,
			Description: "Signal that your turn is complete. Use status='done' when you have finished your work, or status='idle' if there is nothing to do.",
			Parameters: &llm.Schema{
				Type: llm.TypeObject,
				Properties: map[string]*llm.Schema{
					"status": {
						Type:        llm.TypeString,
						Description: "Whether you completed work ('done') or had nothing to do ('idle').",
						Enum:        []string{"done", "idle"},
					},
					"summary": {
						Type:        llm.TypeString,
						Description: "Brief summary of what you accomplished or why you are idle.",
					},
				},
				Required: []string{"status"},
			},
		},
		Fn: func(_ context.Context, _ map[string]any, _ map[string]any) (map[string]any, error) {
			// This should never be called — the ReACT loop intercepts it.
			return nil, fmt.Errorf("end_turn should be intercepted by the reactor")
		},
	}
}
