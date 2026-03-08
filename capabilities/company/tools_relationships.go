package company

import (
	"context"
	"fmt"

	"github.com/dntatme/agents/tool"
)

// ViewRelationshipsTool returns a tool for viewing the caller's relationship scores.
func ViewRelationshipsTool() tool.Tool {
	return tool.Func("view_relationships",
		"View your relationship scores with all other agents. Scores range from -100 to +100 (default 50).").
		Handler(func(_ context.Context, args map[string]any, state map[string]any) (map[string]any, error) {
			caller := GetCurrentAgent(state)
			rl := GetRelationshipLog(state)

			scores := rl.GetAllScores(caller)
			if len(scores) == 0 {
				return map[string]any{
					"relationships": "No relationship scores recorded yet. Default score with everyone is 50 (neutral).",
				}, nil
			}

			rendered := rl.RenderForAgent(caller)
			return map[string]any{
				"relationships": rendered,
			}, nil
		}).
		Build()
}

// UpdateRelationshipTool returns a tool for adjusting a relationship score.
func UpdateRelationshipTool() tool.Tool {
	return tool.Func("update_relationship",
		"Adjust your relationship score with another agent. Use positive delta for improved relations, negative for worsened.").
		StringParam("agent_name", "The agent whose relationship score to adjust.", true).
		IntParam("delta", "Score change from -20 to +20.", true).
		StringParam("reason", "Why you are changing this score.", true).
		Handler(func(_ context.Context, args map[string]any, state map[string]any) (map[string]any, error) {
			agentName, _ := args["agent_name"].(string)
			reason, _ := args["reason"].(string)

			// Handle delta as float64 (JSON numbers) or int
			var delta int
			switch v := args["delta"].(type) {
			case float64:
				delta = int(v)
			case int:
				delta = v
			default:
				return map[string]any{"error": "delta must be a number"}, nil
			}

			caller := GetCurrentAgent(state)
			round := GetCurrentRound(state)
			root := GetWorkspaceRoot(state)
			rl := GetRelationshipLog(state)

			if agentName == caller {
				return map[string]any{"error": "You cannot adjust your relationship with yourself."}, nil
			}

			if delta < -20 || delta > 20 {
				return map[string]any{"error": "Delta must be between -20 and +20."}, nil
			}

			oldScore := rl.GetScore(caller, agentName)
			rl.AdjustScore(caller, agentName, delta, reason, round)
			newScore := rl.GetScore(caller, agentName)

			// Sync relationships file
			if root != "" {
				_ = SyncRelationships(root, rl, caller)
			}

			return map[string]any{
				"status":    "updated",
				"agent":     agentName,
				"old_score": fmt.Sprintf("%d", oldScore),
				"new_score": fmt.Sprintf("%d", newScore),
				"delta":     fmt.Sprintf("%d", delta),
			}, nil
		}).
		Build()
}
