package company

import (
	"context"

	"github.com/dntatme/agents/tool"
)

// LogDecisionTool returns a tool for recording architectural decisions.
func LogDecisionTool() tool.Tool {
	return tool.Func("log_decision", "Record an architectural decision (ADR) with rationale and alternatives considered.").
		StringParam("title", "Short title for the decision.", true).
		StringParam("decision", "What was decided.", true).
		StringParam("rationale", "Why this decision was made.", true).
		StringParam("alternatives", "Other options that were considered.", false).
		Handler(func(_ context.Context, args map[string]any, state map[string]any) (map[string]any, error) {
			title, _ := args["title"].(string)
			decision, _ := args["decision"].(string)
			rationale, _ := args["rationale"].(string)
			alternatives, _ := args["alternatives"].(string)

			dl := GetDecisionLog(state)
			id := dl.Add(title, decision, rationale, alternatives)

			// Sync to file
			root := GetWorkspaceRoot(state)
			if root != "" {
				_ = SyncDecisions(root, dl)
			}

			return map[string]any{"status": "recorded", "decision_id": id}, nil
		}).
		Build()
}

// ReadDecisionsTool returns a tool for reading all architectural decisions.
func ReadDecisionsTool() tool.Tool {
	return tool.Func("read_decisions", "Read all architectural decision records.").
		NoParams().
		Handler(func(_ context.Context, _ map[string]any, state map[string]any) (map[string]any, error) {
			dl := GetDecisionLog(state)
			rendered := dl.Render()
			return map[string]any{"content": rendered}, nil
		}).
		Build()
}
