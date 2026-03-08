package company

import (
	"context"
	"fmt"
	"strings"

	"github.com/dntatme/agents/tool"
)

// FileEscalationTool returns a tool for filing an escalation about another agent.
func FileEscalationTool() tool.Tool {
	return tool.Func("file_escalation",
		"File a formal escalation about a colleague to their manager. This auto-decreases your relationship with the target by -10.").
		StringParam("about_agent", "The agent you are escalating about.", true).
		StringParam("reason", "Why you are filing this escalation.", true).
		StringParam("evidence", "Supporting evidence or examples.", true).
		Handler(func(_ context.Context, args map[string]any, state map[string]any) (map[string]any, error) {
			aboutAgent, _ := args["about_agent"].(string)
			reason, _ := args["reason"].(string)
			evidence, _ := args["evidence"].(string)

			caller := GetCurrentAgent(state)
			round := GetCurrentRound(state)
			root := GetWorkspaceRoot(state)

			oh := GetOrgHierarchy(state)
			if oh == nil {
				return map[string]any{"error": "Org hierarchy not configured."}, nil
			}

			manager := oh.GetManager(aboutAgent)
			if manager == "" {
				return map[string]any{"error": fmt.Sprintf("%s has no manager (cannot escalate about the CEO).", aboutAgent)}, nil
			}
			if manager == caller {
				superior := oh.GetManager(caller)
				if superior == "" {
					return map[string]any{
						"error": fmt.Sprintf("Escalation would route to yourself (%s), and you have no superior.", caller),
					}, nil
				}
				manager = superior
			}

			escLog := GetEscalationLog(state)
			escID := escLog.Add(caller, aboutAgent, manager, reason, evidence, round)

			// Auto-decrease relationship
			rl := GetRelationshipLog(state)
			rl.AdjustScore(caller, aboutAgent, -10, fmt.Sprintf("Filed escalation %s", escID), round)

			// Send email to the manager
			el := GetEmailLog(state)
			subject := fmt.Sprintf("Escalation %s: %s about %s", escID, caller, aboutAgent)
			body := fmt.Sprintf("An escalation has been filed.\n\n"+
				"**Escalation ID:** %s\n"+
				"**Filed by:** %s\n"+
				"**About:** %s\n"+
				"**Reason:** %s\n"+
				"**Evidence:** %s\n\n"+
				"Please review using view_escalations and respond_to_escalation.",
				escID, caller, aboutAgent, reason, evidence)
			el.Send(caller, []string{manager}, nil, subject, body, round, false)

			// Sync files
			if root != "" {
				_ = SyncEscalations(root, escLog)
				_ = SyncRelationships(root, rl, caller)
				_ = SyncInbox(root, el, manager)
			}

			return map[string]any{
				"escalation_id": escID,
				"to_manager":    manager,
				"status":        "filed",
			}, nil
		}).
		Build()
}

// ViewEscalationsTool returns a tool for managers to view escalations filed to them.
func ViewEscalationsTool() tool.Tool {
	return tool.Func("view_escalations",
		"View escalations filed to you as a manager. Only available to managers.").
		StringParam("status_filter", "Filter by status: 'pending' or 'all'. Defaults to 'pending'.", false).
		Handler(func(_ context.Context, args map[string]any, state map[string]any) (map[string]any, error) {
			caller := GetCurrentAgent(state)

			oh := GetOrgHierarchy(state)
			if oh == nil {
				return map[string]any{"error": "Org hierarchy not configured."}, nil
			}
			if !oh.IsManager(caller) {
				return map[string]any{"error": "You are not a manager. Only managers can view escalations."}, nil
			}

			statusFilter, _ := args["status_filter"].(string)
			if statusFilter == "" {
				statusFilter = "pending"
			}

			escLog := GetEscalationLog(state)
			var escalations []Escalation
			if statusFilter == "all" {
				escalations = escLog.GetAllFor(caller)
			} else {
				escalations = escLog.GetPendingFor(caller)
			}

			if len(escalations) == 0 {
				return map[string]any{
					"escalations": fmt.Sprintf("No %s escalations for you.", statusFilter),
				}, nil
			}

			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("# Escalations (%s)\n\n", statusFilter))
			for _, e := range escalations {
				sb.WriteString(fmt.Sprintf("## %s\n\n", e.ID))
				sb.WriteString(fmt.Sprintf("**Filed by:** %s\n", e.FromAgent))
				sb.WriteString(fmt.Sprintf("**About:** %s\n", e.AboutAgent))
				sb.WriteString(fmt.Sprintf("**Reason:** %s\n", e.Reason))
				if e.Evidence != "" {
					sb.WriteString(fmt.Sprintf("**Evidence:** %s\n", e.Evidence))
				}
				sb.WriteString(fmt.Sprintf("**Status:** %s\n", e.Status))
				if e.Resolution != "" {
					sb.WriteString(fmt.Sprintf("**Resolution:** %s\n", e.Resolution))
				}
				sb.WriteString(fmt.Sprintf("**Round:** %d\n\n---\n\n", e.Round))
			}

			return map[string]any{
				"escalations": sb.String(),
				"count":       fmt.Sprintf("%d", len(escalations)),
			}, nil
		}).
		Build()
}

// RespondToEscalationTool returns a tool for managers to respond to escalations.
func RespondToEscalationTool() tool.Tool {
	return tool.Func("respond_to_escalation",
		"Respond to an escalation filed to you. Only available to managers.").
		StringParam("escalation_id", "The escalation ID (e.g. 'ESC-001').", true).
		StringParam("status", "New status: 'acknowledged', 'dismissed', or 'action_taken'.", true).
		StringParam("resolution", "Your resolution or response.", true).
		Handler(func(_ context.Context, args map[string]any, state map[string]any) (map[string]any, error) {
			escalationID, _ := args["escalation_id"].(string)
			status, _ := args["status"].(string)
			resolution, _ := args["resolution"].(string)

			caller := GetCurrentAgent(state)
			round := GetCurrentRound(state)
			root := GetWorkspaceRoot(state)

			oh := GetOrgHierarchy(state)
			if oh == nil {
				return map[string]any{"error": "Org hierarchy not configured."}, nil
			}
			if !oh.IsManager(caller) {
				return map[string]any{"error": "You are not a manager. Only managers can respond to escalations."}, nil
			}

			// Validate status
			validStatuses := map[string]bool{"acknowledged": true, "dismissed": true, "action_taken": true}
			if !validStatuses[status] {
				return map[string]any{"error": "Status must be 'acknowledged', 'dismissed', or 'action_taken'."}, nil
			}

			escLog := GetEscalationLog(state)

			// Check the escalation exists and is assigned to this manager
			esc, found := escLog.GetByID(escalationID)
			if !found {
				return map[string]any{"error": fmt.Sprintf("Escalation %q not found.", escalationID)}, nil
			}
			if esc.ToManager != caller {
				return map[string]any{"error": "This escalation is not assigned to you."}, nil
			}

			if err := escLog.UpdateStatus(escalationID, status, resolution); err != nil {
				return map[string]any{"error": err.Error()}, nil
			}

			// Send email to the original filer
			el := GetEmailLog(state)
			subject := fmt.Sprintf("Re: Escalation %s — %s", escalationID, status)
			body := fmt.Sprintf("Your escalation %s about %s has been updated.\n\n"+
				"**Status:** %s\n"+
				"**Resolution:** %s\n",
				escalationID, esc.AboutAgent, status, resolution)
			el.Send(caller, []string{esc.FromAgent}, nil, subject, body, round, false)

			// Sync files
			if root != "" {
				_ = SyncEscalations(root, escLog)
				_ = SyncInbox(root, el, esc.FromAgent)
			}

			return map[string]any{
				"status":        "responded",
				"escalation_id": escalationID,
			}, nil
		}).
		Build()
}
