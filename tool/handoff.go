package tool

import (
	"context"
	"fmt"

	"github.com/dntatme/agents/llm"
)

const TransferToolName = "transfer_to_agent"

// NewTransferTool creates the transfer_to_agent tool definition.
// agentNames provides the enum of valid target agents.
func NewTransferTool(agentNames []string) Tool {
	return &FuncTool{
		Decl: &llm.FunctionDeclaration{
			Name:        TransferToolName,
			Description: "Transfer the conversation to another specialist agent.",
			Parameters: &llm.Schema{
				Type: llm.TypeObject,
				Properties: map[string]*llm.Schema{
					"agent_name": {
						Type:        llm.TypeString,
						Description: "The name of the agent to transfer to.",
						Enum:        agentNames,
					},
					"reason": {
						Type:        llm.TypeString,
						Description: "Why this transfer is needed.",
					},
				},
				Required: []string{"agent_name"},
			},
		},
		Fn: func(_ context.Context, _ map[string]any, _ map[string]any) (map[string]any, error) {
			// This should never be called — the ReACT loop intercepts it.
			return nil, fmt.Errorf("transfer_to_agent should be intercepted by the reactor")
		},
	}
}
