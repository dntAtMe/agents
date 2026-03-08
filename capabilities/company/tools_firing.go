package company

import (
	"context"
	"fmt"
	"strings"

	"github.com/dntatme/agents/tool"
)

// RequestFireTool returns a tool for managers to request firing an agent.
func RequestFireTool() tool.Tool {
	return tool.Func("request_fire",
		"Request to fire one of your direct reports. If you are CEO, the firing is auto-approved.").
		StringParam("agent_name", "The agent to fire (must be your direct report).", true).
		StringParam("reason", "Reason for the firing request.", true).
		Handler(func(_ context.Context, args map[string]any, state map[string]any) (map[string]any, error) {
			agentName, _ := args["agent_name"].(string)
			reason, _ := args["reason"].(string)

			caller := GetCurrentAgent(state)
			round := GetCurrentRound(state)
			root := GetWorkspaceRoot(state)

			oh := GetOrgHierarchy(state)
			if oh == nil {
				return map[string]any{"error": "Org hierarchy not configured."}, nil
			}
			if !oh.IsManager(caller) {
				return map[string]any{"error": "Only managers can request to fire agents."}, nil
			}

			// Check target is a direct report
			reports := oh.GetDirectReports(caller)
			isReport := false
			for _, r := range reports {
				if r == agentName {
					isReport = true
					break
				}
			}
			if !isReport {
				return map[string]any{"error": fmt.Sprintf("%s is not your direct report.", agentName)}, nil
			}

			fl := GetFiringLog(state)
			el := GetEmailLog(state)
			fireID := fl.RequestFire(agentName, caller, reason, round)

			// CEO auto-approves
			if caller == "ceo" {
				_ = fl.CEODecision(fireID, "approved", "Auto-approved by CEO.")
				firedAgents := GetFiredAgents(state)
				firedAgents[agentName] = true

				// Notify the fired agent
				el.Send("ceo", []string{agentName},
					fmt.Sprintf("Termination Notice — %s", fireID),
					fmt.Sprintf("You have been terminated by the CEO.\n\n**Reason:** %s\n\nThis decision is final.", reason),
					round)

				if root != "" {
					_ = SyncFirings(root, fl)
					_ = SyncInbox(root, el, agentName)
				}

				return map[string]any{
					"fire_id":  fireID,
					"status":   "approved",
					"message":  fmt.Sprintf("%s has been fired (CEO auto-approved).", agentName),
				}, nil
			}

			// Non-CEO: send request to CEO
			el.Send(caller, []string{"ceo"},
				fmt.Sprintf("Firing Request %s: %s wants to fire %s", fireID, caller, agentName),
				fmt.Sprintf("A firing request has been submitted.\n\n"+
					"**Request ID:** %s\n"+
					"**Target:** %s\n"+
					"**Requested by:** %s\n"+
					"**Reason:** %s\n\n"+
					"Please review using view_fire_requests and approve_fire.",
					fireID, agentName, caller, reason),
				round)

			if root != "" {
				_ = SyncFirings(root, fl)
				_ = SyncInbox(root, el, "ceo")
			}

			return map[string]any{
				"fire_id": fireID,
				"status":  "pending_ceo_approval",
				"message": fmt.Sprintf("Firing request for %s sent to CEO for approval.", agentName),
			}, nil
		}).
		Build()
}

// ViewFireRequestsTool returns a tool for the CEO to view pending firing requests.
func ViewFireRequestsTool() tool.Tool {
	return tool.Func("view_fire_requests",
		"View pending firing requests. CEO only.").
		Handler(func(_ context.Context, args map[string]any, state map[string]any) (map[string]any, error) {
			caller := GetCurrentAgent(state)
			if caller != "ceo" {
				return map[string]any{"error": "Only the CEO can view firing requests."}, nil
			}

			fl := GetFiringLog(state)
			pending := fl.GetPendingApprovals()

			if len(pending) == 0 {
				return map[string]any{
					"fire_requests": "No pending firing requests.",
				}, nil
			}

			var sb strings.Builder
			sb.WriteString("# Pending Firing Requests\n\n")
			for _, r := range pending {
				sb.WriteString(fmt.Sprintf("## %s\n\n", r.ID))
				sb.WriteString(fmt.Sprintf("**Target:** %s\n", r.TargetAgent))
				sb.WriteString(fmt.Sprintf("**Requested by:** %s\n", r.RequestedBy))
				sb.WriteString(fmt.Sprintf("**Reason:** %s\n", r.Reason))
				sb.WriteString(fmt.Sprintf("**Round:** %d\n\n---\n\n", r.Round))
			}

			return map[string]any{
				"fire_requests": sb.String(),
				"count":         fmt.Sprintf("%d", len(pending)),
			}, nil
		}).
		Build()
}

// ApproveFireTool returns a tool for the CEO to approve or deny firing requests.
func ApproveFireTool() tool.Tool {
	return tool.Func("approve_fire",
		"Approve or deny a firing request. CEO only.").
		StringParam("fire_id", "The firing request ID (e.g. 'FIRE-001').", true).
		StringParam("decision", "Your decision: 'approved' or 'denied'.", true).
		StringParam("comments", "Your comments on the decision.", true).
		Handler(func(_ context.Context, args map[string]any, state map[string]any) (map[string]any, error) {
			fireID, _ := args["fire_id"].(string)
			decision, _ := args["decision"].(string)
			comments, _ := args["comments"].(string)

			caller := GetCurrentAgent(state)
			round := GetCurrentRound(state)
			root := GetWorkspaceRoot(state)

			if caller != "ceo" {
				return map[string]any{"error": "Only the CEO can approve firing requests."}, nil
			}

			validDecisions := map[string]bool{"approved": true, "denied": true}
			if !validDecisions[decision] {
				return map[string]any{"error": "Decision must be 'approved' or 'denied'."}, nil
			}

			fl := GetFiringLog(state)

			// Find the request to get target info before updating
			pending := fl.GetPendingApprovals()
			var targetAgent, requestedBy string
			for _, r := range pending {
				if r.ID == fireID {
					targetAgent = r.TargetAgent
					requestedBy = r.RequestedBy
					break
				}
			}
			if targetAgent == "" {
				return map[string]any{"error": fmt.Sprintf("Firing request %q not found or already decided.", fireID)}, nil
			}

			if err := fl.CEODecision(fireID, decision, comments); err != nil {
				return map[string]any{"error": err.Error()}, nil
			}

			el := GetEmailLog(state)

			if decision == "approved" {
				firedAgents := GetFiredAgents(state)
				firedAgents[targetAgent] = true

				// Notify requester
				el.Send("ceo", []string{requestedBy},
					fmt.Sprintf("Firing Request %s Approved", fireID),
					fmt.Sprintf("Your request to fire %s has been approved.\n\n**Comments:** %s", targetAgent, comments),
					round)

				// Notify fired agent
				el.Send("ceo", []string{targetAgent},
					fmt.Sprintf("Termination Notice — %s", fireID),
					fmt.Sprintf("You have been terminated.\n\n**Reason:** Per firing request %s.\n**CEO Comments:** %s\n\nThis decision is final.", fireID, comments),
					round)

				if root != "" {
					_ = SyncInbox(root, el, requestedBy)
					_ = SyncInbox(root, el, targetAgent)
				}
			} else {
				// Denied — notify requester
				el.Send("ceo", []string{requestedBy},
					fmt.Sprintf("Firing Request %s Denied", fireID),
					fmt.Sprintf("Your request to fire %s has been denied.\n\n**CEO Comments:** %s", targetAgent, comments),
					round)

				if root != "" {
					_ = SyncInbox(root, el, requestedBy)
				}
			}

			if root != "" {
				_ = SyncFirings(root, fl)
			}

			return map[string]any{
				"status":   decision,
				"fire_id":  fireID,
				"target":   targetAgent,
			}, nil
		}).
		Build()
}
