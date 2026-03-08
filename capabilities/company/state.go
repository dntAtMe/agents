package company

// State keys used throughout the company simulation.
const (
	KeyWorkspaceRoot  = "workspace_root"
	KeyCurrentAgent   = "current_agent"
	KeyCurrentRound   = "current_round"
	KeyProjectName    = "project_name"
	KeyProjectStatus  = "project_status"
	KeyTasks          = "tasks"
	KeyDecisions      = "decisions"
	KeyUpdates        = "updates"
	KeyAgentLastRound = "agent_last_round"
	KeySimRuntime     = "sim_runtime"
)

// GetTaskBoard retrieves or creates the TaskBoard in shared state.
func GetTaskBoard(state map[string]any) *TaskBoard {
	if v, ok := state[KeyTasks]; ok {
		if tb, ok := v.(*TaskBoard); ok {
			return tb
		}
	}
	tb := NewTaskBoard()
	state[KeyTasks] = tb
	return tb
}

// GetDecisionLog retrieves or creates the DecisionLog in shared state.
func GetDecisionLog(state map[string]any) *DecisionLog {
	if v, ok := state[KeyDecisions]; ok {
		if dl, ok := v.(*DecisionLog); ok {
			return dl
		}
	}
	dl := NewDecisionLog()
	state[KeyDecisions] = dl
	return dl
}

// GetUpdateLog retrieves or creates the UpdateLog in shared state.
func GetUpdateLog(state map[string]any) *UpdateLog {
	if v, ok := state[KeyUpdates]; ok {
		if ul, ok := v.(*UpdateLog); ok {
			return ul
		}
	}
	ul := NewUpdateLog()
	state[KeyUpdates] = ul
	return ul
}

// GetCurrentAgent returns the current agent name from state.
func GetCurrentAgent(state map[string]any) string {
	if v, ok := state[KeyCurrentAgent]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// GetCurrentRound returns the current round number from state.
func GetCurrentRound(state map[string]any) int {
	if v, ok := state[KeyCurrentRound]; ok {
		switch n := v.(type) {
		case int:
			return n
		case float64:
			return int(n)
		}
	}
	return 0
}

// GetAgentLastRound returns the map tracking each agent's last active round.
func GetAgentLastRound(state map[string]any) map[string]int {
	if v, ok := state[KeyAgentLastRound]; ok {
		if m, ok := v.(map[string]int); ok {
			return m
		}
	}
	m := make(map[string]int)
	state[KeyAgentLastRound] = m
	return m
}

// GetSimRuntime returns the SimRuntime from state, or nil if not set.
// The return type is any to avoid a circular dependency with the agent package.
// Callers must type-assert to *agent.SimRuntime.
func GetSimRuntime(state map[string]any) any {
	return state[KeySimRuntime]
}

// GetWorkspaceRoot returns the workspace root path from state.
func GetWorkspaceRoot(state map[string]any) string {
	if v, ok := state[KeyWorkspaceRoot]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
