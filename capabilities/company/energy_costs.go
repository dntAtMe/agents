package company

// ToolCosts maps tool names to their action point cost.
var ToolCosts = map[string]int{
	"read_task_board":       1,
	"read_file":             1,
	"list_files":            1,
	"read_updates":          1,
	"read_decisions":        1,
	"check_inbox":           1,
	"view_relationships":    1,
	"view_escalations":      1,
	"view_fire_requests":    1,
	"write_diary":           2,
	"post_update":           2,
	"reply_email":           2,
	"update_task":           2,
	"update_relationship":   2,
	"respond_to_escalation": 2,
	"send_email":            3,
	"write_file":            3,
	"append_to_file":        3,
	"add_task":              3,
	"write_review":          3,
	"log_decision":          3,
	"file_escalation":       3,
	"record_pip":            3,
	"request_fire":          3,
	"approve_fire":          3,
	"get_coffee":            3,
	"call_group_meeting":    5,
	"start_interview":       5,
	"hire_decision":         3,
	"update_stock_price":    3,
	"check_stock_price":     1,
}

// GetToolCost returns the AP cost for a tool. Unknown tools default to 2.
func GetToolCost(toolName string) int {
	if cost, ok := ToolCosts[toolName]; ok {
		return cost
	}
	return 2
}

// AllowedToolsCoffeeBreak is the set of tools available during a coffee break.
var AllowedToolsCoffeeBreak = map[string]bool{
	"view_relationships":  true,
	"update_relationship": true,
}

// AllowedToolsUrgentEmail is the set of tools available when an agent
// is woken up out of turn by an urgent email.
var AllowedToolsUrgentEmail = map[string]bool{
	// Read-only (gather context)
	"check_inbox":        true,
	"read_file":          true,
	"list_files":         true,
	"read_task_board":    true,
	"read_updates":       true,
	"read_decisions":     true,
	"view_relationships": true,
	"view_escalations":   true,
	"view_fire_requests": true,
	// Respond
	"reply_email": true,
	// Reflect
	"write_diary": true,
}

// KeyAllowedTools is the state key for the current tool restriction set.
// When non-nil (map[string]bool), only listed tools may be called.
const KeyAllowedTools = "allowed_tools"

// SetAllowedTools sets a tool restriction on shared state.
// Pass nil to remove the restriction.
func SetAllowedTools(state map[string]any, allowed map[string]bool) {
	if allowed == nil {
		delete(state, KeyAllowedTools)
	} else {
		state[KeyAllowedTools] = allowed
	}
}

// GetAllowedTools returns the current tool restriction, or nil if unrestricted.
func GetAllowedTools(state map[string]any) map[string]bool {
	if v, ok := state[KeyAllowedTools]; ok {
		if m, ok := v.(map[string]bool); ok {
			return m
		}
	}
	return nil
}
