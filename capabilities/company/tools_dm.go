package company

import (
	"context"
	"fmt"

	"github.com/dntatme/agents/tool"
)

// AskAgentTool returns a tool for directly messaging another agent and getting
// an immediate response. The target agent runs a mini-turn with the message
// and its response is returned as the tool result.
func AskAgentTool() tool.Tool {
	return tool.Func("ask_agent",
		"Send a direct message to another agent and get an immediate response. "+
			"The target agent will receive your message, use its tools to research if needed, "+
			"and respond directly. Use this for quick clarifications, questions, or feedback requests. "+
			"Max 3 DMs per agent per round.").
		StringParam("agent_name", "The name of the agent to message (e.g. 'architect', 'cto', 'backend-dev').", true).
		StringParam("message", "The question or request to send to the target agent.", true).
		Handler(func(ctx context.Context, args map[string]any, state map[string]any) (map[string]any, error) {
			targetName, _ := args["agent_name"].(string)
			message, _ := args["message"].(string)

			caller := GetCurrentAgent(state)
			round := GetCurrentRound(state)

			// Guardrail: cannot DM yourself
			if targetName == caller {
				return map[string]any{
					"error": "You cannot send a direct message to yourself.",
				}, nil
			}

			// Guardrail: max 3 DMs per agent per round
			dmKey := fmt.Sprintf("dm_count_%s_%d", caller, round)
			dmCount := 0
			if v, ok := state[dmKey]; ok {
				switch n := v.(type) {
				case int:
					dmCount = n
				case float64:
					dmCount = int(n)
				}
			}
			if dmCount >= 3 {
				return map[string]any{
					"error": fmt.Sprintf("DM limit reached: you have already sent %d direct messages this round (max 3).", dmCount),
				}, nil
			}

			// Get the RunAgent function stored in state by the simulator
			runAgentFn, ok := state["sim_run_agent"].(func(ctx context.Context, targetName, message string, state map[string]any) (string, error))
			if !ok {
				return map[string]any{
					"error": "Direct messaging is not available outside of simulation.",
				}, nil
			}

			// Build the DM prompt
			dmPrompt := fmt.Sprintf(
				"[Direct message from %s in round %d]: %s\n\n"+
					"Respond directly and concisely to this message. You may use your tools if needed to look up information.",
				caller, round, message,
			)

			// Save and restore current_agent
			state[KeyCurrentAgent] = targetName
			response, err := runAgentFn(ctx, targetName, dmPrompt, state)
			state[KeyCurrentAgent] = caller

			// Increment DM count
			state[dmKey] = dmCount + 1

			if err != nil {
				return map[string]any{
					"error": fmt.Sprintf("Failed to reach %s: %v", targetName, err),
				}, nil
			}

			return map[string]any{
				"response": response,
				"from":     targetName,
			}, nil
		}).
		Build()
}
