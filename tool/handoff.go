package tool

import (
	"context"
	"fmt"

	"google.golang.org/genai"
)

const TransferToolName = "transfer_to_agent"

// NewTransferTool creates the transfer_to_agent tool definition.
// agentNames provides the enum of valid target agents.
func NewTransferTool(agentNames []string) Tool {
	return &FuncTool{
		Decl: &genai.FunctionDeclaration{
			Name:        TransferToolName,
			Description: "Transfer the conversation to another specialist agent.",
			Parameters: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"agent_name": {
						Type:        genai.TypeString,
						Description: "The name of the agent to transfer to.",
						Enum:        agentNames,
					},
					"reason": {
						Type:        genai.TypeString,
						Description: "Why this transfer is needed.",
					},
				},
				Required: []string{"agent_name"},
			},
		},
		Fn: func(_ context.Context, _ map[string]any) (map[string]any, error) {
			// This should never be called — the ReACT loop intercepts it.
			return nil, fmt.Errorf("transfer_to_agent should be intercepted by the reactor")
		},
	}
}
