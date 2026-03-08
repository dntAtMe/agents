package company

import (
	"context"
	"fmt"

	"github.com/dntatme/agents/tool"
)

// RecordPiPTool returns a tool for managers to record a Performance Improvement Plan.
func RecordPiPTool() tool.Tool {
	return tool.Func("record_pip",
		"Record a Performance Improvement Plan (PiP) for an agent in your management chain.").
		StringParam("agent_name", "The agent to place on PiP (must be in your hierarchy chain).", true).
		StringParam("reason", "Why this PiP is being issued.", true).
		StringParam("expectations", "Specific improvements expected from the agent.", true).
		IntParam("review_round", "Round to review PiP progress (optional, defaults to current round + 2).", false).
		Handler(func(_ context.Context, args map[string]any, state map[string]any) (map[string]any, error) {
			agentName, _ := args["agent_name"].(string)
			reason, _ := args["reason"].(string)
			expectations, _ := args["expectations"].(string)

			caller := GetCurrentAgent(state)
			round := GetCurrentRound(state)
			root := GetWorkspaceRoot(state)

			oh := GetOrgHierarchy(state)
			if oh == nil {
				return map[string]any{"error": "Org hierarchy not configured."}, nil
			}
			if !oh.IsManager(caller) {
				return map[string]any{"error": "Only managers can record PiPs."}, nil
			}
			if !oh.IsInManagementChain(caller, agentName) {
				return map[string]any{
					"error": fmt.Sprintf("%s is not in your management chain.", agentName),
				}, nil
			}

			reviewRound := round + 2
			if v, ok := args["review_round"]; ok {
				switch n := v.(type) {
				case int:
					reviewRound = n
				case float64:
					reviewRound = int(n)
				}
			}
			if reviewRound <= round {
				return map[string]any{"error": "review_round must be greater than the current round."}, nil
			}

			pl := GetPiPLog(state)
			pipID := pl.Add(agentName, caller, reason, expectations, reviewRound, round)

			// Notify the target agent.
			el := GetEmailLog(state)
			subject := fmt.Sprintf("Performance Improvement Plan %s", pipID)
			body := fmt.Sprintf("A Performance Improvement Plan has been issued.\n\n"+
				"**PiP ID:** %s\n"+
				"**Target:** %s\n"+
				"**Recorded by:** %s\n"+
				"**Reason:** %s\n"+
				"**Expectations:** %s\n"+
				"**Review round:** %d\n",
				pipID, agentName, caller, reason, expectations, reviewRound)
			el.Send(caller, []string{agentName}, subject, body, round)

			if root != "" {
				_ = SyncPiPs(root, pl)
				_ = SyncInbox(root, el, agentName)
			}

			return map[string]any{
				"pip_id":       pipID,
				"target_agent": agentName,
				"review_round": fmt.Sprintf("%d", reviewRound),
				"status":       "recorded",
			}, nil
		}).
		Build()
}
