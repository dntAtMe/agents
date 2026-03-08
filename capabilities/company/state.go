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
	KeyEmails         = "emails"
	KeyMeetings       = "meetings"
	KeyRelationships  = "relationships"
	KeyEscalations    = "escalations"
	KeyPIPs           = "pips"
	KeyFirings        = "firings"
	KeyOrgHierarchy   = "org_hierarchy"
	KeyFiredAgents    = "fired_agents"
	KeyCodeReviews    = "code_reviews"
	KeyFileSnapshots  = "file_snapshots"
	KeyCommandLog     = "command_log"
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

// GetEmailLog retrieves or creates the EmailLog in shared state.
func GetEmailLog(state map[string]any) *EmailLog {
	if v, ok := state[KeyEmails]; ok {
		if el, ok := v.(*EmailLog); ok {
			return el
		}
	}
	el := NewEmailLog()
	state[KeyEmails] = el
	return el
}

// GetMeetingLog retrieves or creates the MeetingLog in shared state.
func GetMeetingLog(state map[string]any) *MeetingLog {
	if v, ok := state[KeyMeetings]; ok {
		if ml, ok := v.(*MeetingLog); ok {
			return ml
		}
	}
	ml := NewMeetingLog()
	state[KeyMeetings] = ml
	return ml
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

// GetRelationshipLog retrieves or creates the RelationshipLog in shared state.
func GetRelationshipLog(state map[string]any) *RelationshipLog {
	if v, ok := state[KeyRelationships]; ok {
		if rl, ok := v.(*RelationshipLog); ok {
			return rl
		}
	}
	rl := NewRelationshipLog()
	state[KeyRelationships] = rl
	return rl
}

// GetEscalationLog retrieves or creates the EscalationLog in shared state.
func GetEscalationLog(state map[string]any) *EscalationLog {
	if v, ok := state[KeyEscalations]; ok {
		if el, ok := v.(*EscalationLog); ok {
			return el
		}
	}
	el := NewEscalationLog()
	state[KeyEscalations] = el
	return el
}

// GetPiPLog retrieves or creates the PiPLog in shared state.
func GetPiPLog(state map[string]any) *PiPLog {
	if v, ok := state[KeyPIPs]; ok {
		if pl, ok := v.(*PiPLog); ok {
			return pl
		}
	}
	pl := NewPiPLog()
	state[KeyPIPs] = pl
	return pl
}

// GetFiringLog retrieves or creates the FiringLog in shared state.
func GetFiringLog(state map[string]any) *FiringLog {
	if v, ok := state[KeyFirings]; ok {
		if fl, ok := v.(*FiringLog); ok {
			return fl
		}
	}
	fl := NewFiringLog()
	state[KeyFirings] = fl
	return fl
}

// GetOrgHierarchy retrieves the OrgHierarchy from shared state, or nil if not set.
func GetOrgHierarchy(state map[string]any) *OrgHierarchy {
	if v, ok := state[KeyOrgHierarchy]; ok {
		if oh, ok := v.(*OrgHierarchy); ok {
			return oh
		}
	}
	return nil
}

// GetCodeReviewLog retrieves or creates the CodeReviewLog in shared state.
func GetCodeReviewLog(state map[string]any) *CodeReviewLog {
	if v, ok := state[KeyCodeReviews]; ok {
		if cl, ok := v.(*CodeReviewLog); ok {
			return cl
		}
	}
	cl := NewCodeReviewLog()
	state[KeyCodeReviews] = cl
	return cl
}

// GetFileSnapshotLog retrieves or creates the FileSnapshotLog in shared state.
func GetFileSnapshotLog(state map[string]any) *FileSnapshotLog {
	if v, ok := state[KeyFileSnapshots]; ok {
		if fl, ok := v.(*FileSnapshotLog); ok {
			return fl
		}
	}
	fl := NewFileSnapshotLog()
	state[KeyFileSnapshots] = fl
	return fl
}

// GetCommandLog retrieves or creates the CommandLog in shared state.
func GetCommandLog(state map[string]any) *CommandLog {
	if v, ok := state[KeyCommandLog]; ok {
		if cl, ok := v.(*CommandLog); ok {
			return cl
		}
	}
	cl := NewCommandLog()
	state[KeyCommandLog] = cl
	return cl
}

// GetFiredAgents returns the map of fired agents from state.
func GetFiredAgents(state map[string]any) map[string]bool {
	if v, ok := state[KeyFiredAgents]; ok {
		if m, ok := v.(map[string]bool); ok {
			return m
		}
	}
	m := make(map[string]bool)
	state[KeyFiredAgents] = m
	return m
}
