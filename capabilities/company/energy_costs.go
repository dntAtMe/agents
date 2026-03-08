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
}

// GetToolCost returns the AP cost for a tool. Unknown tools default to 2.
func GetToolCost(toolName string) int {
	if cost, ok := ToolCosts[toolName]; ok {
		return cost
	}
	return 2
}
